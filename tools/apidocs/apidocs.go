// Finds the packages, reads their docs and writes one Markdown page each.

package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// skip lists folders that never get API pages.
var skip = map[string]bool{"docs": true, "test": true, "site": true, "testdata": true, "vendor": true}

// Page is one generated API page.
type Page struct {
	Dir     string // package folder, like "internal/engine"
	File    string // page file name, like "internal-engine.md"
	Summary string // first sentence of the package doc
	Body    string // the Markdown
}

// Generate writes one page per package plus an index into out, and returns
// how many package pages it wrote. Old pages in out are removed first, so a
// deleted package never leaves a stale page behind.
func Generate(root, out string) (int, error) {
	pages, err := Build(root)
	if err != nil {
		return 0, err
	}
	dir := filepath.Join(root, out)
	if err := os.RemoveAll(dir); err != nil {
		return 0, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, err
	}
	for _, p := range pages {
		if err := os.WriteFile(filepath.Join(dir, p.File), []byte(p.Body), 0o644); err != nil {
			return 0, err
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(Index(pages)), 0o644); err != nil {
		return 0, err
	}
	return len(pages), nil
}

// Build reads every package under root and returns its page, sorted by folder.
func Build(root string) ([]Page, error) {
	module, err := readModule(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil, err
	}
	var pages []Page
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		if path != root && (skip[name] || strings.HasPrefix(name, ".")) {
			return filepath.SkipDir
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if path != root && skip[strings.Split(rel, "/")[0]] {
			return filepath.SkipDir
		}
		p, ok, err := buildPage(root, rel, module)
		if err != nil {
			return err
		}
		if ok {
			pages = append(pages, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(pages, func(a, b Page) int { return strings.Compare(a.Dir, b.Dir) })
	return pages, nil
}

// buildPage makes the page for one folder. ok is false when the folder has
// no Go package (only test files, or no Go files at all).
func buildPage(root, rel, module string) (Page, bool, error) {
	fset := token.NewFileSet()
	dir := filepath.Join(root, rel)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Page{}, false, err
	}
	var files []*ast.File
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, n), nil, parser.ParseComments)
		if err != nil {
			return Page{}, false, fmt.Errorf("%s/%s: %w", rel, n, err)
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		return Page{}, false, nil
	}

	importPath := module
	if rel != "." {
		importPath += "/" + rel
	}
	pkg, err := doc.NewFromFiles(fset, files, importPath)
	if err != nil {
		return Page{}, false, fmt.Errorf("%s: %w", rel, err)
	}
	name := rel
	if rel == "." {
		name = pkg.Name
	}
	r := &renderer{fset: fset, pkg: pkg, root: root, repoURL: readRepoURL(root)}
	return Page{
		Dir:     name,
		File:    strings.ReplaceAll(name, "/", "-") + ".md",
		Summary: summary(pkg.Name, pkg.Doc),
		Body:    r.page(name, importPath),
	}, true, nil
}

// Index is the API reference front page: a table of every package.
func Index(pages []Page) string {
	var b strings.Builder
	b.WriteString("# API reference\n\n")
	b.WriteString("Every package in Nuqs, built from the comments in the code, so it's always up to date. ")
	b.WriteString("To change something here, change the comment in the code.\n\n")
	b.WriteString("| Package | What it does |\n|---|---|\n")
	for _, p := range pages {
		fmt.Fprintf(&b, "| [%s](%s) | %s |\n", p.Dir, p.File, strings.ReplaceAll(p.Summary, "|", "\\|"))
	}
	return b.String()
}

// renderer turns one package's docs into Markdown.
type renderer struct {
	fset    *token.FileSet
	pkg     *doc.Package
	root    string
	repoURL string // like "https://github.com/nuqs-org/nuqs", or "" for no links
	b       strings.Builder
}

func (r *renderer) w(format string, a ...any) { fmt.Fprintf(&r.b, format, a...) }

func (r *renderer) page(name, importPath string) string {
	p := r.pkg
	r.w("# %s\n\n", name)
	r.w("```go\nimport \"%s\"\n```\n\n", importPath)
	r.text(p.Doc)

	if p.Name == "main" {
		r.w("!!! note\n    This is a program, not a library, so it has no API to list.\n")
		return r.b.String()
	}

	if len(p.Consts) > 0 {
		r.w("## Constants\n\n")
		for _, v := range p.Consts {
			r.value(v)
		}
	}
	if len(p.Vars) > 0 {
		r.w("## Variables\n\n")
		for _, v := range p.Vars {
			r.value(v)
		}
	}
	if len(p.Funcs) > 0 {
		r.w("## Functions\n\n")
		for _, f := range p.Funcs {
			r.fn("###", f.Name, f)
		}
	}
	if len(p.Types) > 0 {
		r.w("## Types\n\n")
		for _, t := range p.Types {
			r.w("### %s\n\n", t.Name)
			r.text(t.Doc)
			r.code(t.Decl)
			r.where(t.Decl)
			for _, v := range t.Consts {
				r.value(v)
			}
			for _, v := range t.Vars {
				r.value(v)
			}
			for _, f := range t.Funcs {
				r.fn("####", f.Name, f)
			}
			for _, m := range t.Methods {
				r.fn("####", t.Name+"."+m.Name, m)
			}
		}
	}
	return r.b.String()
}

func (r *renderer) fn(level, title string, f *doc.Func) {
	r.w("%s %s\n\n", level, title)
	r.text(f.Doc)
	decl := *f.Decl
	decl.Body = nil // signature only
	decl.Doc = nil
	r.code(&decl)
	r.where(f.Decl)
}

func (r *renderer) value(v *doc.Value) {
	r.text(v.Doc)
	r.code(v.Decl)
	r.where(v.Decl)
}

// code prints a declaration exactly as Go would format it.
func (r *renderer) code(node any) {
	var buf bytes.Buffer
	cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 4}
	if gd, ok := node.(*ast.GenDecl); ok {
		cp := *gd
		cp.Doc = nil
		node = &cp
	}
	if err := cfg.Fprint(&buf, r.fset, node); err != nil {
		buf.Reset()
		buf.WriteString("// (could not print this declaration)")
	}
	r.w("```go\n%s\n```\n\n", buf.String())
}

// where says which file and line a declaration lives in, linked to GitHub
// when mkdocs.yml has a repo_url.
func (r *renderer) where(n ast.Node) {
	pos := r.fset.Position(n.Pos())
	rel, err := filepath.Rel(r.root, pos.Filename)
	if err != nil {
		return
	}
	rel = filepath.ToSlash(rel)
	if r.repoURL == "" {
		r.w("<small>Defined in `%s`</small>\n\n", rel)
		return
	}
	r.w("<small>Defined in [`%s:%d`](%s/blob/HEAD/%s#L%d)</small>\n\n", rel, pos.Line, r.repoURL, rel, pos.Line)
}

// text renders a doc comment as Markdown, so lists, code blocks and
// headings in comments come out properly on the website.
func (r *renderer) text(s string) {
	if strings.TrimSpace(s) == "" {
		return
	}
	pr := r.pkg.Printer()
	pr.HeadingLevel = 4
	r.w("%s\n", pr.Markdown(r.pkg.Parser().Parse(s)))
}

// summary returns the first sentence of a doc comment, without the
// "Package x" or "Command x is" at the start, since the name is shown next
// to it. (tools/archdiagram has the same small function.)
func summary(pkgName, doc string) string {
	doc = strings.Join(strings.Fields(doc), " ")
	if i := strings.Index(doc, ". "); i >= 0 {
		doc = doc[:i+1]
	}
	prefixes := []string{"Package " + pkgName + " "}
	if f := strings.Fields(doc); pkgName == "main" && len(f) > 2 && f[0] == "Command" {
		prefixes = append(prefixes, "Command "+f[1]+" ")
	}
	for _, prefix := range prefixes {
		if rest, ok := strings.CutPrefix(doc, prefix); ok && rest != "" {
			rest = strings.TrimPrefix(rest, "is ")
			return strings.ToUpper(rest[:1]) + rest[1:]
		}
	}
	return doc
}

// readRepoURL reads repo_url from mkdocs.yml, the one place the repo's
// address is written down. It returns "" if there isn't one.
func readRepoURL(root string) string {
	b, err := os.ReadFile(filepath.Join(root, "mkdocs.yml"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(line, "repo_url:"); ok {
			return strings.TrimSuffix(strings.Trim(strings.TrimSpace(v), `"'`), "/")
		}
	}
	return ""
}

func readModule(gomod string) (string, error) {
	f, err := os.Open(gomod)
	if err != nil {
		return "", fmt.Errorf("can't read go.mod (run this from the repo root): %w", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if m, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "module "); ok {
			return strings.TrimSpace(m), nil
		}
	}
	return "", errors.New("go.mod has no module line")
}
