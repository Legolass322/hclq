package query_test

import (
	"testing"

	"github.com/Legolass322/hclq/internal/query"
)

func TestEval(t *testing.T) {
	in := map[string]any{
		"resource": map[string]any{
			"aws_instance": map[string]any{
				"web": map[string]any{"ami": "ami-123"},
			},
		},
	}
	got, err := query.Eval(in, ".resource.aws_instance.web.ami")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "ami-123" {
		t.Fatalf("got %#v", got)
	}
}

func TestEvalIterate(t *testing.T) {
	in := map[string]any{"n": []any{1, 2}}
	got, err := query.Eval(in, ".n[]")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("got %#v", got)
	}
}

func TestParseError(t *testing.T) {
	_, err := query.Eval(nil, ".[")
	if err == nil {
		t.Fatal("expected error")
	}
}
