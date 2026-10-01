// Package convert reads HCL and JSON into a JSON-shaped value and writes
// values back as HCL expressions.
//
// A block becomes a nested object keyed by its type and labels. Attributes
// are keys on the nearest object. Several blocks at the same path become an
// array of bodies. An expression that needs variables or functions is left
// as its source text.
package convert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// Parse converts src into a value suitable for encoding/json.
// filename is used only in error messages. A document that starts with '{'
// or '[' is parsed as JSON. Anything else is parsed as native HCL.
func Parse(filename string, src []byte) (any, error) {
	if filename == "" {
		filename = "stdin"
	}
	trimmed := bytes.TrimSpace(src)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("%s: empty input", filename)
	}
	switch trimmed[0] {
	case '{', '[':
		return parseJSON(filename, trimmed)
	default:
		return parseHCL(filename, src)
	}
}

func parseJSON(filename string, src []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(src))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("%s: %w", filename, err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("%s: extra value after JSON", filename)
		}
		return nil, fmt.Errorf("%s: %w", filename, err)
	}
	return v, nil
}

func parseHCL(filename string, src []byte) (any, error) {
	file, diags := hclsyntax.ParseConfig(src, filename, hcl.InitialPos)
	if diags.HasErrors() {
		return nil, fmt.Errorf("%s", strings.TrimSpace(diags.Error()))
	}
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil, fmt.Errorf("%s: unsupported HCL body", filename)
	}
	return convertBody(body, src)
}

func convertBody(body *hclsyntax.Body, src []byte) (map[string]any, error) {
	out := make(map[string]any)
	for name, attr := range body.Attributes {
		v, err := convertExpr(attr.Expr, src)
		if err != nil {
			return nil, err
		}
		out[name] = v
	}
	for _, block := range body.Blocks {
		child, err := convertBody(block.Body, src)
		if err != nil {
			return nil, err
		}
		path := make([]string, 0, 1+len(block.Labels))
		path = append(path, block.Type)
		path = append(path, block.Labels...)
		if err := insertBlock(out, path, child); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// insertBlock places body at path. A path that already holds a body becomes
// an array. Descending through an array is an error: the same path was used
// both as a leaf and as a parent.
func insertBlock(node map[string]any, path []string, body map[string]any) error {
	key := path[0]
	if len(path) == 1 {
		if existing, ok := node[key]; ok {
			node[key] = appendBody(existing, body)
			return nil
		}
		node[key] = body
		return nil
	}

	next, ok := node[key]
	if !ok {
		child := make(map[string]any)
		node[key] = child
		return insertBlock(child, path[1:], body)
	}
	child, ok := next.(map[string]any)
	if !ok {
		return fmt.Errorf("block %s conflicts with an earlier %s", strings.Join(path, " "), key)
	}
	return insertBlock(child, path[1:], body)
}

func appendBody(existing any, body map[string]any) any {
	if list, ok := existing.([]any); ok {
		return append(list, body)
	}
	return []any{existing, body}
}

func convertExpr(expr hclsyntax.Expression, src []byte) (any, error) {
	val, diags := expr.Value(nil)
	if diags.HasErrors() || !val.IsKnown() {
		return string(expr.Range().SliceBytes(src)), nil
	}
	v, err := ctyToGo(val)
	if err != nil {
		return string(expr.Range().SliceBytes(src)), nil
	}
	return v, nil
}

func ctyToGo(v cty.Value) (any, error) {
	if v.IsNull() {
		return nil, nil
	}
	if !v.IsKnown() {
		return nil, fmt.Errorf("unknown value")
	}

	t := v.Type()
	switch {
	case t == cty.String:
		return v.AsString(), nil
	case t == cty.Bool:
		return v.True(), nil
	case t == cty.Number:
		return json.Number(v.AsBigFloat().Text('f', -1)), nil
	case t.IsListType() || t.IsTupleType() || t.IsSetType():
		out := make([]any, 0, v.LengthInt())
		for it := v.ElementIterator(); it.Next(); {
			_, ev := it.Element()
			gv, err := ctyToGo(ev)
			if err != nil {
				return nil, err
			}
			out = append(out, gv)
		}
		return out, nil
	case t.IsMapType() || t.IsObjectType():
		out := make(map[string]any)
		for it := v.ElementIterator(); it.Next(); {
			kv, ev := it.Element()
			gv, err := ctyToGo(ev)
			if err != nil {
				return nil, err
			}
			out[kv.AsString()] = gv
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported type %s", t.FriendlyName())
	}
}
