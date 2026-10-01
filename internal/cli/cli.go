// Package cli is the hclq command-line interface.
package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Legolass322/hclq/internal/convert"
	"github.com/Legolass322/hclq/internal/query"
)

const usageText = `usage: hclq [flags] [filter] [file...]

Query HCL files with a jq filter. Reads stdin when no files are given.
A piped invocation with no filter uses ".".

  -c           compact JSON, one value per line
  -e           exit 1 if the last result is false or null
  -f file      read the filter from a file
  -H           print HCL expressions instead of JSON
  -n           do not read input; the filter receives null
  -r           print strings without JSON quotes
  -s           read every input into one array
  -version     print the version

Exit status is 0 on success, 1 when -e rejects the last result, 2 on a usage
error, 3 when the filter does not compile, 4 when input does not parse, and
5 when the filter fails at runtime.
`

const (
	exitOK       = 0
	exitRejected = 1
	exitUsage    = 2
	exitCompile  = 3
	exitParse    = 4
	exitRuntime  = 5
)

// Options is a parsed invocation. Stdin, Stdout, and Stderr must be set.
type Options struct {
	Compact    bool
	Raw        bool
	ExitStatus bool
	NullInput  bool
	Slurp      bool
	HCL        bool
	Program    string
	Files      []string

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Main parses args and runs the command. version is printed by -version.
func Main(args []string, version string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("hclq", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), usageText)
	}

	opt := Options{Stdin: stdin, Stdout: stdout, Stderr: stderr}
	var progFile string
	var showVersion bool
	fs.BoolVar(&opt.Compact, "c", false, "compact JSON output")
	fs.BoolVar(&opt.Raw, "r", false, "raw string output")
	fs.BoolVar(&opt.ExitStatus, "e", false, "exit 1 on false or null")
	fs.BoolVar(&opt.NullInput, "n", false, "use null input")
	fs.BoolVar(&opt.Slurp, "s", false, "slurp inputs into an array")
	fs.BoolVar(&opt.HCL, "H", false, "print HCL")
	fs.StringVar(&progFile, "f", "", "read filter from file")
	fs.BoolVar(&showVersion, "version", false, "print version")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	if showVersion {
		fmt.Fprintln(stdout, version)
		return exitOK
	}

	rest := fs.Args()
	switch {
	case progFile != "":
		b, err := os.ReadFile(progFile)
		if err != nil {
			fmt.Fprintf(stderr, "hclq: %s\n", err)
			return exitUsage
		}
		opt.Program = string(b)
		opt.Files = rest
	case len(rest) == 0:
		opt.Program = "."
		if !opt.NullInput && isTerminal(stdin) {
			fs.Usage()
			return exitUsage
		}
	default:
		opt.Program = rest[0]
		opt.Files = rest[1:]
	}
	opt.Program = strings.TrimSpace(opt.Program)
	if opt.Program == "" {
		fmt.Fprintln(stderr, "hclq: empty filter")
		return exitCompile
	}
	return Run(opt)
}

// Run executes a parsed invocation.
func Run(opt Options) int {
	prog, err := query.Compile(opt.Program)
	if err != nil {
		return filterError(opt, err)
	}

	inputs, code := readInputs(opt)
	if code != exitOK {
		return code
	}

	var results []any
	for _, input := range inputs {
		part, err := prog.Run(input)
		if err != nil {
			return filterError(opt, err)
		}
		results = append(results, part...)
	}

	if err := writeResults(opt, results); err != nil {
		fmt.Fprintf(opt.Stderr, "hclq: %s\n", err)
		return exitRuntime
	}
	if opt.ExitStatus && !accepted(results) {
		return exitRejected
	}
	return exitOK
}

func readInputs(opt Options) ([]any, int) {
	if opt.NullInput {
		return []any{nil}, exitOK
	}

	files := opt.Files
	if len(files) == 0 {
		files = []string{"-"}
	}

	var values []any
	for _, name := range files {
		src, err := readFile(name, opt.Stdin)
		if err != nil {
			fmt.Fprintf(opt.Stderr, "hclq: %s\n", err)
			return nil, exitParse
		}
		filename := name
		if name == "-" {
			filename = "stdin"
		}
		v, err := convert.Parse(filename, src)
		if err != nil {
			fmt.Fprintf(opt.Stderr, "%s\n", err)
			return nil, exitParse
		}
		values = append(values, v)
	}
	if opt.Slurp {
		return []any{values}, exitOK
	}
	return values, exitOK
}

func readFile(name string, stdin io.Reader) ([]byte, error) {
	if name == "-" {
		b, err := io.ReadAll(stdin)
		if err != nil {
			return nil, fmt.Errorf("stdin: %w", err)
		}
		return b, nil
	}
	b, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func filterError(opt Options, err error) int {
	if isCompileError(err) {
		fmt.Fprintf(opt.Stderr, "hclq: filter: %s\n", err)
		if looksLikeFile(opt.Program) {
			fmt.Fprintf(opt.Stderr, "hclq: the first argument is the filter; try: hclq . %s\n", opt.Program)
		}
		return exitCompile
	}
	fmt.Fprintf(opt.Stderr, "hclq: %s\n", err)
	return exitRuntime
}

func isCompileError(err error) bool {
	var compile *query.CompileError
	return errors.As(err, &compile)
}

func looksLikeFile(path string) bool {
	if path == "" || strings.ContainsAny(path, " \t\n|") {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func writeResults(opt Options, results []any) error {
	for i, v := range results {
		if err := writeOne(opt, v); err != nil {
			return err
		}
		if opt.HCL && i != len(results)-1 {
			if _, err := io.WriteString(opt.Stdout, "\n"); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeOne(opt Options, v any) error {
	if opt.HCL {
		b, err := convert.Format(v)
		if err != nil {
			return err
		}
		_, err = opt.Stdout.Write(b)
		return err
	}
	if opt.Raw {
		if s, ok := v.(string); ok {
			_, err := io.WriteString(opt.Stdout, s)
			if err != nil {
				return err
			}
			_, err = io.WriteString(opt.Stdout, "\n")
			return err
		}
	}
	var (
		b   []byte
		err error
	)
	if opt.Compact {
		b, err = json.Marshal(v)
	} else {
		b, err = json.MarshalIndent(v, "", "  ")
	}
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = opt.Stdout.Write(b)
	return err
}

func accepted(results []any) bool {
	if len(results) == 0 {
		return false
	}
	last := results[len(results)-1]
	if last == nil {
		return false
	}
	falsy, ok := last.(bool)
	return !ok || falsy
}

func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
