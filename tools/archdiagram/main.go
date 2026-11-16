// Command archdiagram writes docs/architecture.md, a diagram and a map of the
// repo built straight from the code.
//
// It reads every Go file's package doc comment and imports, and lists every
// file in the repo. Nothing is typed by hand, so the page can't drift from
// the code. CI runs it on every push and commits the result if it changed.
//
// Run it from the repo root:
//
//	go run ./tools/archdiagram          # write docs/architecture.md
//	go run ./tools/archdiagram -check   # only check it's up to date
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	root := flag.String("root", ".", "repo root")
	check := flag.Bool("check", false, "don't write, exit 1 if the page is out of date")
	flag.Parse()
	os.Exit(run(*root, *check))
}

func run(root string, check bool) int {
	repo, err := Scan(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "archdiagram:", err)
		return 1
	}
	page := []byte(Render(repo))
	path := filepath.Join(root, outFile)

	if check {
		old, _ := os.ReadFile(path)
		if !bytes.Equal(old, page) {
			fmt.Fprintf(os.Stderr, "%s is out of date, run: go run ./tools/archdiagram\n", outFile)
			return 1
		}
		fmt.Println(outFile, "is up to date")
		return 0
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "archdiagram:", err)
		return 1
	}
	if err := os.WriteFile(path, page, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "archdiagram:", err)
		return 1
	}
	fmt.Println("wrote", outFile)
	return 0
}
