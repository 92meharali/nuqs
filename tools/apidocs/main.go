// Command apidocs writes the API reference pages of the website, one page
// per package, from the doc comments in the code.
//
// Each type and function gets its own heading, so the website's sidebar works
// as a clickable index. Run it from the repo root:
//
//	go run ./tools/apidocs        # writes docs/api/
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	root := flag.String("root", ".", "repo root")
	out := flag.String("out", "docs/api", "where to write the pages")
	flag.Parse()

	n, err := Generate(*root, *out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "apidocs:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %d API pages to %s\n", n, *out)
}
