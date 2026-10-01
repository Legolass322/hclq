package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Legolass322/hclq/internal/cli"
)

const sample = `
resource "aws_instance" "web" {
  ami = "ami-123"
}
`

func TestMainQuery(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cli.Main(
		[]string{".resource.aws_instance.web.ami"},
		"dev",
		strings.NewReader(sample),
		&stdout,
		&stderr,
	)
	if code != 0 {
		t.Fatalf("code %d stderr %s", code, stderr.String())
	}
	if stdout.String() != "\"ami-123\"\n" {
		t.Fatalf("stdout %q", stdout.String())
	}
}

func TestRaw(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cli.Main(
		[]string{"-r", ".resource.aws_instance.web.ami"},
		"dev",
		strings.NewReader(sample),
		&stdout,
		&stderr,
	)
	if code != 0 {
		t.Fatalf("code %d stderr %s", code, stderr.String())
	}
	if stdout.String() != "ami-123\n" {
		t.Fatalf("stdout %q", stdout.String())
	}
}

func TestSlurp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	in := strings.NewReader("a = 1\n")
	code := cli.Run(cli.Options{
		Program: ".[0].a",
		Slurp:   true,
		Stdin:   in,
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if code != 0 {
		t.Fatalf("code %d stderr %s", code, stderr.String())
	}
	if stdout.String() != "1\n" {
		t.Fatalf("stdout %q", stdout.String())
	}
}

func TestBadFilter(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cli.Main([]string{".["}, "dev", strings.NewReader(sample), &stdout, &stderr)
	if code != 3 {
		t.Fatalf("code %d stderr %s", code, stderr.String())
	}
}

func TestUndefinedFunction(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cli.Main([]string{"nosuch"}, "dev", strings.NewReader(sample), &stdout, &stderr)
	if code != 3 {
		t.Fatalf("code %d stderr %s", code, stderr.String())
	}
}

func TestFileUsedAsFilter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.hcl")
	if err := os.WriteFile(path, []byte("a = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := cli.Main([]string{path}, "dev", strings.NewReader(""), &stdout, &stderr)
	if code != 3 {
		t.Fatalf("code %d stderr %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "first argument is the filter") {
		t.Fatalf("stderr %s", stderr.String())
	}
}

func TestBadInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cli.Main([]string{"."}, "dev", strings.NewReader(`resource "x" {`), &stdout, &stderr)
	if code != 4 {
		t.Fatalf("code %d stderr %s", code, stderr.String())
	}
}

func TestExitStatus(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cli.Run(cli.Options{
		Program:    ".missing",
		ExitStatus: true,
		Stdin:      strings.NewReader("a = 1\n"),
		Stdout:     &stdout,
		Stderr:     &stderr,
	})
	if code != 1 {
		t.Fatalf("code %d stderr %s stdout %s", code, stderr.String(), stdout.String())
	}
}

func TestVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cli.Main([]string{"-version"}, "1.2.3", strings.NewReader(""), &stdout, &stderr)
	if code != 0 || stdout.String() != "1.2.3\n" {
		t.Fatalf("code %d stdout %q stderr %s", code, stdout.String(), stderr.String())
	}
}

func TestHCLOut(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cli.Run(cli.Options{
		Program: ".resource.aws_instance.web",
		HCL:     true,
		Stdin:   strings.NewReader(sample),
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if code != 0 {
		t.Fatalf("code %d stderr %s", code, stderr.String())
	}
	if stdout.String() != "ami = \"ami-123\"\n" {
		t.Fatalf("stdout %q", stdout.String())
	}
}
