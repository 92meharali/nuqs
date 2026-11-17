// Tests for the API page generator, run against a small fake module.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func build(t *testing.T) map[string]Page {
	t.Helper()
	pages, err := Build("testdata/mod")
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]Page{}
	for _, p := range pages {
		m[p.Dir] = p
	}
	return m
}

func TestWhichPackagesGetPages(t *testing.T) {
	pages := build(t)
	if len(pages) != 2 {
		t.Errorf("expected pages for shapes and cmd/draw only, got %d: %v", len(pages), pages)
	}
	if _, ok := pages["shapes"]; !ok {
		t.Error("shapes should have a page")
	}
	if _, ok := pages["cmd/draw"]; !ok {
		t.Error("cmd/draw should have a page")
	}
	if _, ok := pages["onlytests"]; ok {
		t.Error("a folder with only tests has no API")
	}
	if _, ok := pages["docs/inner"]; ok {
		t.Error("docs/ must be skipped")
	}
}

func TestPageContent(t *testing.T) {
	p := build(t)["shapes"]
	if p.File != "shapes.md" || p.Summary != "Draws simple shapes." {
		t.Errorf("file %q, summary %q", p.File, p.Summary)
	}
	body := p.Body
	for _, want := range []string{
		"# shapes\n",
		"import \"example.com/demo/shapes\"",
		"- squares", // lists in comments become real Markdown lists
		"s := shapes.NewSquare(2)",
		"## Constants",
		"const Max = 100",
		"## Variables",
		"var Count int",
		"## Functions",
		"### Area\n\nArea returns the area of any shape.",
		"func Area(s Shape) float64\n```", // signature only, no body
		"## Types",
		"### Square\n\nSquare is a square.", // description comes before the code
		"Side float64 // length of one side",
		"#### NewSquare", // constructors sit under their type
		"#### Square.Area",
		"func (s *Square) Area() float64\n```",
		"KindCircle Kind = \"circle\"", // typed constants sit under their type
		"[`shapes/shapes.go:13`](https://github.com/example/demo/blob/HEAD/shapes/shapes.go#L13)",
		"shapes/extra.go", // declarations from a second file are linked to that file
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page is missing %q\n\n%s", want, body)
		}
	}
	for _, hidden := range []string{"secret int", "hidden", "helper", "return s.Side"} {
		if strings.Contains(body, hidden) {
			t.Errorf("page shows %q, but unexported things and function bodies must stay hidden", hidden)
		}
	}
	if strings.Index(body, "### Area") > strings.Index(body, "### Square") {
		t.Error("functions should come before types")
	}
}

func TestCommandPage(t *testing.T) {
	p := build(t)["cmd/draw"]
	if p.Summary != "Draws shapes on the terminal." {
		t.Errorf("summary = %q", p.Summary)
	}
	if strings.Contains(p.Body, "Exported") || !strings.Contains(p.Body, "This is a program") {
		t.Errorf("a command page shows its description only:\n%s", p.Body)
	}
}

func TestIndex(t *testing.T) {
	idx := Index([]Page{{Dir: "a/b", File: "a-b.md", Summary: "Does | things."}})
	for _, want := range []string{"# API reference", "| [a/b](a-b.md) | Does \\| things. |"} {
		if !strings.Contains(idx, want) {
			t.Errorf("index is missing %q:\n%s", want, idx)
		}
	}
}

func TestGenerateRemovesOldPages(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/x\n"), 0o644)
	os.MkdirAll(filepath.Join(root, "p"), 0o755)
	os.WriteFile(filepath.Join(root, "p", "p.go"), []byte("// Package p is a test.\npackage p\n\n// F does nothing.\nfunc F() {}\n"), 0o644)
	os.MkdirAll(filepath.Join(root, "out"), 0o755)
	os.WriteFile(filepath.Join(root, "out", "deleted-package.md"), []byte("old"), 0o644)

	n, err := Generate(root, "out")
	if err != nil || n != 1 {
		t.Fatalf("Generate = %d, %v", n, err)
	}
	if _, err := os.Stat(filepath.Join(root, "out", "deleted-package.md")); err == nil {
		t.Error("pages for deleted packages must be removed")
	}
	for _, f := range []string{"index.md", "p.md"} {
		if _, err := os.Stat(filepath.Join(root, "out", f)); err != nil {
			t.Errorf("%s not written", f)
		}
	}
	b, _ := os.ReadFile(filepath.Join(root, "out", "p.md"))
	if !strings.Contains(string(b), "<small>Defined in `p/p.go`</small>") {
		t.Errorf("without mkdocs.yml, file names are shown without links:\n%s", b)
	}
}

func TestErrors(t *testing.T) {
	if _, err := Build(t.TempDir()); err == nil {
		t.Error("no go.mod should fail")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/x\n"), 0o644)
	os.WriteFile(filepath.Join(root, "bad.go"), []byte("package x\nfunc ("), 0o644)
	if _, err := Build(root); err == nil {
		t.Error("broken Go code must fail loudly, not give wrong docs")
	}
	noModule := t.TempDir()
	os.WriteFile(filepath.Join(noModule, "go.mod"), []byte("go 1.24\n"), 0o644)
	if _, err := Build(noModule); err == nil {
		t.Error("go.mod without a module line should fail")
	}
}

// Every real package in Nuqs gets a page.
func TestRealRepo(t *testing.T) {
	pages, err := Build("../..")
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) < 10 {
		t.Errorf("only %d pages for the real repo, something is off", len(pages))
	}
	for _, p := range pages {
		if p.Summary == "" {
			t.Errorf("package %s has no doc comment", p.Dir)
		}
	}
}

func TestSummary(t *testing.T) {
	cases := []struct{ pkg, doc, want string }{
		{"engine", "Package engine runs a scan. More.", "Runs a scan."},
		{"cli", "Package cli is the CLI.", "The CLI."},
		{"main", "Command nuqs is the scanner.", "The scanner."},
		{"x", "Other text.", "Other text."},
	}
	for _, c := range cases {
		if got := summary(c.pkg, c.doc); got != c.want {
			t.Errorf("summary(%q) = %q, want %q", c.doc, got, c.want)
		}
	}
}
