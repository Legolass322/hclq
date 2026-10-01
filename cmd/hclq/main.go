// Command hclq queries HCL documents with jq filters.
package main

import (
	"os"

	"github.com/Legolass322/hclq/internal/cli"
)

// version is set with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(cli.Main(os.Args[1:], version, os.Stdin, os.Stdout, os.Stderr))
}
