package convert_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Legolass322/hclq/internal/convert"
)

func TestParseHCL(t *testing.T) {
	src := []byte(`
resource "aws_instance" "web" {
  ami   = "ami-123"
  count = 2
  tags  = { Name = "web" }
}

resource "aws_instance" "db" {
  ami = "ami-9"
}

ingress {
  port = 80
}
ingress {
  port = 443
}

locals {
  name = var.region
  n    = 1 + 2
}
`)
	got, err := convert.Parse("test.hcl", src)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}

	var pretty map[string]any
	if err := json.Unmarshal(b, &pretty); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"resource": map[string]any{
			"aws_instance": map[string]any{
				"web": map[string]any{
					"ami":   "ami-123",
					"count": float64(2),
					"tags":  map[string]any{"Name": "web"},
				},
				"db": map[string]any{"ami": "ami-9"},
			},
		},
		"ingress": []any{
			map[string]any{"port": float64(80)},
			map[string]any{"port": float64(443)},
		},
		"locals": map[string]any{
			"name": "var.region",
			"n":    float64(3),
		},
	}
	if !jsonEqual(t, pretty, want) {
		t.Fatalf("got %#v", pretty)
	}
}

func TestParseJSON(t *testing.T) {
	got, err := convert.Parse("in.json", []byte(`{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("got %T", got)
	}
	if m["a"] != json.Number("1") {
		t.Fatalf("got %#v", m["a"])
	}
}

func TestParseError(t *testing.T) {
	_, err := convert.Parse("bad.hcl", []byte(`resource "x" {`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFormat(t *testing.T) {
	got, err := convert.Format(map[string]any{
		"name":    "web",
		"count":   json.Number("2"),
		"enabled": true,
		"tags":    map[string]any{"Name": "web"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `count = 2 enabled = true name = "web" tags = { Name = "web" }`
	if strings.Join(strings.Fields(string(got)), " ") != want {
		t.Fatalf("got:\n%s", got)
	}
}

func jsonEqual(t *testing.T, got, want any) bool {
	t.Helper()
	gb, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	wb, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	return string(gb) == string(wb)
}
