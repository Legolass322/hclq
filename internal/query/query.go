// Package query evaluates jq filters against JSON-shaped values.
package query

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/itchyny/gojq"
)

// CompileError is returned when a filter cannot be parsed or compiled.
type CompileError struct{ Err error }

func (e *CompileError) Error() string { return e.Err.Error() }

func (e *CompileError) Unwrap() error { return e.Err }

// Program is a compiled filter.
type Program struct {
	code *gojq.Code
}

// Compile parses and compiles a jq filter.
func Compile(program string) (*Program, error) {
	q, err := gojq.Parse(program)
	if err != nil {
		return nil, &CompileError{Err: err}
	}
	code, err := gojq.Compile(q)
	if err != nil {
		return nil, &CompileError{Err: err}
	}
	return &Program{code: code}, nil
}

// Eval runs program against input and returns every result.
func Eval(input any, program string) ([]any, error) {
	p, err := Compile(program)
	if err != nil {
		return nil, err
	}
	return p.Run(input)
}

// Run evaluates the compiled filter against input.
func (p *Program) Run(input any) ([]any, error) {
	var out []any
	iter := p.code.Run(input)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			if halt, ok := err.(*gojq.HaltError); ok && halt.Value() == nil {
				break
			}
			return nil, err
		}
		nv, err := normalize(v)
		if err != nil {
			return nil, err
		}
		out = append(out, nv)
	}
	return out, nil
}

func normalize(v any) (any, error) {
	switch t := v.(type) {
	case nil, bool, string, json.Number:
		return v, nil
	case int:
		return t, nil
	case int64:
		return t, nil
	case float64:
		return t, nil
	case *big.Int:
		return json.Number(t.String()), nil
	case []any:
		for i, el := range t {
			nv, err := normalize(el)
			if err != nil {
				return nil, err
			}
			t[i] = nv
		}
		return t, nil
	case map[string]any:
		for k, el := range t {
			nv, err := normalize(el)
			if err != nil {
				return nil, err
			}
			t[k] = nv
		}
		return t, nil
	default:
		return nil, fmt.Errorf("unsupported result %T", v)
	}
}
