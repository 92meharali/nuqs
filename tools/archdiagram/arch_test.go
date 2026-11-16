package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// copyFixture copies testdata/repo to a temp folder (not a git repo), so
// tests can write to it and the folder-walking fallback gets used.
func copyFixture(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.Walk("testdata/repo", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel("testdata/repo", path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func TestScanFixture(t *testing.T) {
	r, err := Scan(copyFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if r.Module != "example.com/demo" {
		t.Errorf("module = %q", r.Module)
	}
	want := []Package{
		{Dir: "a", Summary: "Holds the shared types."},
		{Dir: "b", Summary: "The middle layer.", Imports: []string{"a"}},
		{Dir: "checks/x", Summary: "Checks X | with a pipe.", Imports: []string{"a"}},
		{Dir: "checks/y", Imports: []string{"a", "b"}},
		{Dir: "cmd/app", Summary: "The demo program.", Imports: []string{"b"}},
		{Dir: "test/e2e", Summary: "Runs everything end to end.", Imports: []string{"b"}, TestOnly: true},
	}
	if !reflect.DeepEqual(r.Packages, want) {
		t.Errorf("packages:\n got %+v\nwant %+v", r.Packages, want)
	}
	// Every file shows up in the tree, including testdata and the page itself.
	for _, f := range []string{"a/testdata/fixture.go", "docs/index.md", "go.mod", outFile} {
		if !slices.Contains(r.Files, f) {
			t.Errorf("file %s is missing from the tree", f)
		}
	}
	if !slices.IsSorted(r.Files) {
		t.Error("files must be sorted")
	}
}

func TestRenderFixture(t *testing.T) {
	r, err := Scan(copyFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	page := Render(r)
	for _, want := range []string{
		"```mermaid\nflowchart LR\n",
		`n_checks__["checks/*<br/><small>x, y</small>"]`, // check packages collapse into one box
		"n_b --> n_a",
		"n_checks__ --> n_a",
		"n_checks__ --> n_b",
		"n_cmd_app --> n_b",
		"n_test_e2e -.-> n_b", // test-only packages get dotted arrows
		"Dotted arrows come from test-only packages.",
		"| `b` | The middle layer. | `a` |",
		"| `checks/x` | Checks X \\| with a pipe. | `a` |", // pipes can't break the table
		"| `checks/y` | - | `a`, `b` |",
		"| `a` | Holds the shared types. | nothing else in Nuqs |",
		"| `test/e2e` (tests) |",
		"├── a/                            # Holds the shared types.",
		"│   ├── testdata/                 # Sample files used by the tests next to it.",
		"├── cmd/                          # Programs you can run.",
		"└── go.mod",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page is missing %q\n\n%s", want, page)
		}
	}
	if strings.Count(page, "n_checks__ --> n_a") != 1 {
		t.Error("collapsed boxes must not repeat the same arrow")
	}
	if strings.Contains(page, "n_checks__ --> n_checks__") {
		t.Error("no arrows from a box to itself")
	}
}

func TestRunWritesAndChecks(t *testing.T) {
	root := copyFixture(t)
	if code := run(root, true); code != 1 {
		t.Error("-check must fail before the page exists")
	}
	if code := run(root, false); code != 0 {
		t.Fatal("writing failed")
	}
	if code := run(root, true); code != 0 {
		t.Error("-check must pass right after writing (the output must be stable)")
	}
	first, _ := os.ReadFile(filepath.Join(root, outFile))
	run(root, false)
	second, _ := os.ReadFile(filepath.Join(root, outFile))
	if string(first) != string(second) {
		t.Error("running twice must give the same page")
	}

	// Adding a package must make the page out of date.
	os.MkdirAll(filepath.Join(root, "c"), 0o755)
	os.WriteFile(filepath.Join(root, "c", "c.go"), []byte("// Package c is new.\npackage c\n"), 0o644)
	if code := run(root, true); code != 1 {
		t.Error("-check must notice a new package")
	}
}

func TestErrors(t *testing.T) {
	if _, err := Scan(t.TempDir()); err == nil {
		t.Error("a folder without go.mod should fail")
	}
	if code := run(t.TempDir(), false); code != 1 {
		t.Error("run should fail without go.mod")
	}

	root := copyFixture(t)
	os.WriteFile(filepath.Join(root, "b", "broken.go"), []byte("package b\nimport ("), 0o644)
	if _, err := Scan(root); err == nil {
		t.Error("a broken Go file should fail loudly, not give a wrong picture")
	}

	noModule := t.TempDir()
	os.WriteFile(filepath.Join(noModule, "go.mod"), []byte("go 1.24\n"), 0o644)
	if _, err := Scan(noModule); err == nil {
		t.Error("go.mod without a module line should fail")
	}
}

func TestSummary(t *testing.T) {
	cases := []struct{ pkg, doc, want string }{
		{"engine", "Package engine runs a scan.\nMore.", "Runs a scan."},
		{"cli", "Package cli is the \"nuqs\" command line. More.", "The \"nuqs\" command line."},
		{"main", "Command nuqs is the scanner.", "The scanner."},
		{"main", "Command archdiagram writes a page. More.", "Writes a page."},
		{"x", "Something else entirely.", "Something else entirely."},
		{"x", "", ""},
		{"x", "Package x", "Package x"},
	}
	for _, c := range cases {
		if got := summary(c.pkg, c.doc); got != c.want {
			t.Errorf("summary(%q, %q) = %q, want %q", c.pkg, c.doc, got, c.want)
		}
	}
}

// TestMatchesGoList is the accuracy guarantee. It compares what the
// generator found in the real Nuqs repo with what the Go tool itself
// reports. If they ever disagree, the diagram would be wrong, so we fail.
func TestMatchesGoList(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool not found")
	}
	root := "../.."
	r, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	type listed struct {
		ImportPath string
		Imports    []string
		GoFiles    []string
	}
	fromGo := map[string][]string{}
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for dec.More() {
		var p listed
		if err := dec.Decode(&p); err != nil {
			t.Fatal(err)
		}
		if len(p.GoFiles) == 0 {
			continue // test-only folders, checked below
		}
		dir := strings.TrimPrefix(strings.TrimPrefix(p.ImportPath, r.Module), "/")
		var imps []string
		for _, imp := range p.Imports {
			if strings.HasPrefix(imp, r.Module+"/") {
				imps = append(imps, strings.TrimPrefix(imp, r.Module+"/"))
			}
		}
		fromGo[dir] = imps
	}

	fromUs := map[string][]string{}
	for _, p := range r.Packages {
		if !p.TestOnly {
			fromUs[p.Dir] = p.Imports
		}
	}
	if !reflect.DeepEqual(fromUs, fromGo) {
		t.Errorf("the diagram disagrees with go list\nours:    %v\ngo list: %v", fromUs, fromGo)
	}
	if len(fromGo) < 10 {
		t.Errorf("go list only found %d packages, something is off", len(fromGo))
	}
}
