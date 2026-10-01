package convert

import (
	"encoding/json"
	"fmt"
	"maps"
	"math/big"
	"slices"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

// Format renders v as HCL. A top-level object becomes a body of attributes.
// Nested objects are HCL object expressions, so the result matches the JSON
// view of the document rather than the original blocks.
func Format(v any) ([]byte, error) {
	obj, ok := v.(map[string]any)
	if !ok {
		cv, err := toCty(v)
		if err != nil {
			return nil, err
		}
		out := hclwrite.Format(hclwrite.TokensForValue(cv).Bytes())
		return ensureNL(out), nil
	}

	f := hclwrite.NewEmptyFile()
	body := f.Body()
	for _, key := range slices.Sorted(maps.Keys(obj)) {
		cv, err := toCty(obj[key])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		body.SetAttributeValue(key, cv)
	}
	return ensureNL(hclwrite.Format(f.Bytes())), nil
}

func ensureNL(b []byte) []byte {
	if len(b) == 0 || b[len(b)-1] != '\n' {
		return append(b, '\n')
	}
	return b
}

func toCty(v any) (cty.Value, error) {
	switch t := v.(type) {
	case nil:
		return cty.NullVal(cty.DynamicPseudoType), nil
	case bool:
		return cty.BoolVal(t), nil
	case string:
		return cty.StringVal(t), nil
	case json.Number:
		return cty.ParseNumberVal(string(t))
	case int:
		return cty.NumberIntVal(int64(t)), nil
	case int64:
		return cty.NumberIntVal(t), nil
	case float64:
		return cty.NumberFloatVal(t), nil
	case *big.Int:
		return cty.ParseNumberVal(t.String())
	case []any:
		if len(t) == 0 {
			return cty.EmptyTupleVal, nil
		}
		vals := make([]cty.Value, len(t))
		for i, el := range t {
			cv, err := toCty(el)
			if err != nil {
				return cty.NilVal, err
			}
			vals[i] = cv
		}
		return cty.TupleVal(vals), nil
	case map[string]any:
		attrs := make(map[string]cty.Value, len(t))
		for _, key := range slices.Sorted(maps.Keys(t)) {
			cv, err := toCty(t[key])
			if err != nil {
				return cty.NilVal, fmt.Errorf("%s: %w", key, err)
			}
			attrs[key] = cv
		}
		return cty.ObjectVal(attrs), nil
	default:
		return cty.NilVal, fmt.Errorf("cannot format %T as HCL", v)
	}
}
