// Tests the generator, and checks it against go list so the diagram is always right.

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
		{Dir: "smoke", Summary: "Runs quick checks of the whole demo.", Imports: []string{"b"}, TestOnly: true},
	}
	if !reflect.DeepEqual(r.Packages, want) {
		t.Errorf("packages:\n got %+v\nwant %+v", r.Packages, want)
	}
	// Every file shows up in the tree, including testdata.
	for _, f := range []string{"a/testdata/fixture.go", "smoke/smoke_test.go", "go.mod"} {
		if !slices.Contains(r.Files, f) {
			t.Errorf("file %s is missing from the tree", f)
		}
	}
	// docs/ and test/ are left out completely.
	for _, f := range r.Files {
		if strings.HasPrefix(f, "docs/") || strings.HasPrefix(f, "test/") {
			t.Errorf("%s should be left out", f)
		}
	}
	if !slices.IsSorted(r.Files) {
		t.Error("files must be sorted")
	}
}

// useFixtureLayers swaps in layers for the testdata repo for one test.
func useFixtureLayers(t *testing.T) {
	t.Helper()
	old := layers
	layers = []Layer{
		{"Programs", []string{"cmd/*"}},
		{"Checks", []string{"checks/*"}},
		{"Middle", []string{"b"}},
		{"Base", []string{"a", "c"}},
	}
	t.Cleanup(func() { layers = old })
}

func TestLayerProblems(t *testing.T) {
	ls := []Layer{{"Top", []string{"internal/cli"}}, {"Bottom", []string{"internal/model"}}}
	r := &Repo{Packages: []Package{
		{Dir: "internal/cli", Imports: []string{"internal/model"}},
		{Dir: "internal/model", Imports: []string{"internal/cli"}}, // uses a layer above
		{Dir: "internal/new"},          // in no layer
		{Dir: "tools/gen"},             // dev tools don't need a layer
		{Dir: "smoke", TestOnly: true}, // neither do test-only packages
	}}
	got := LayerProblems(r, ls)
	if len(got) != 2 ||
		!strings.Contains(got[0], "internal/model (Bottom) uses internal/cli (Top)") ||
		!strings.Contains(got[1], "internal/new isn't in a layer") {
		t.Errorf("problems: %q", got)
	}
}

// TestRealRepoLayers keeps the real diagram honest: every Nuqs package is
// in a layer, and none uses a layer above its own.
func TestRealRepoLayers(t *testing.T) {
	r, err := Scan("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range LayerProblems(r, layers) {
		t.Error(p)
	}
}

func TestDiagramSkipsDevFolders(t *testing.T) {
	r := &Repo{Packages: []Package{
		{Dir: "internal/model"},
		{Dir: "internal/cli", Imports: []string{"internal/model"}},
		{Dir: "tools/gen", Imports: []string{"internal/model"}},
		{Dir: "lab", Imports: []string{"internal/model"}},
	}}
	var b strings.Builder
	renderDiagram(&b, r, []Layer{{"Top", []string{"internal/cli", "lab", "tools/gen"}}, {"Empty", nil}, {"Bottom", []string{"internal/model"}}})
	d := b.String()
	if strings.Contains(d, "gen") || strings.Contains(d, "n_lab") || strings.Contains(d, "Empty") || !strings.Contains(d, "n_internal_cli ~~~ n_internal_model") || !strings.Contains(d, "layer0 --> layer2") {
		t.Errorf("dev folders and empty layers should be left out:\n%s", d)
	}
}

func TestRenderFixture(t *testing.T) {
	r, err := Scan(copyFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	useFixtureLayers(t)
	page := Render(r)
	for _, want := range []string{
		"```mermaid\nflowchart TB\n",
		"subgraph layer0 [\"Programs\"]\n",
		`n_checks__["<b>checks/*</b><br/><small>x, y</small>"]`, // check packages collapse into one box
		`n_a["<b>a</b>"]`,                                       // a top-level package isn't repeated as its path
		`n_cmd_app["<b>app</b><br/><small>cmd/app</small>"]`,
		"n_cmd_app ~~~ n_checks__\n  n_checks__ ~~~ n_b\n  n_b ~~~ n_a\n", // keeps the rows in order
		"layer0 --> layer1\n  layer1 --> layer2\n  layer2 --> layer3\n",   // one arrow from each row to the next
		"style layer0 ",
		"classDef cmd ", "class n_cmd_app cmd",
		"classDef checks ", "class n_checks__ checks",
		", and test-only packages, aren't drawn.",
		"| `b` | The middle layer. | `a` |",
		"| `checks/x` | Checks X \\| with a pipe. | `a` |", // pipes can't break the table
		"| `checks/y` | - | `a`, `b` |",
		"| `a` | Holds the shared types. | nothing else in Nuqs |",
		"| `smoke` (tests) | Runs quick checks of the whole demo. | `b` |",
		"<li><code>go.mod</code>",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page is missing %q\n\n%s", want, page)
		}
	}
	// Tree lines: the right note on the right line (spacing doesn't matter).
	for _, pair := range [][2]string{
		{"<summary><code>a/</code>", "Holds the shared types."},
		{`<li class="folder"><code>testdata/</code>`, "Sample files used by the tests next to it (1 file)."},
		{"<summary><code>cmd/</code>", "Programs you can run."},
		{"<li><code>b.go</code>", ""}, // b.go holds the package doc, so it doesn't repeat the folder's note
	} {
		if !hasLine(page, pair[0], pair[1]) {
			t.Errorf("no tree line with %q and %q\n\n%s", pair[0], pair[1], page)
		}
	}
	// The tree must be valid HTML that opens and closes.
	for open, closing := range map[string]string{"<details>": "</details>", "<ul>": "</ul>", "<li": "</li>"} {
		if o, c := strings.Count(page, open), strings.Count(page, closing); o == 0 || o != c {
			t.Errorf("%d %s but %d %s", o, open, c, closing)
		}
	}
	if strings.Contains(page[strings.Index(page, `<div class="file-tree">`):], "\n\n") {
		t.Error("the tree has a blank line, Markdown would break it")
	}
	if strings.Contains(page, "fixture.go") {
		t.Error("files inside testdata/ should be collapsed into one line")
	}
	// docs/ and test/ must not appear in the diagram, the table or the tree.
	for _, gone := range []string{"e2e", "index.md", "<code>docs/</code>", "<code>test/</code>", "n_test_", "| `test/"} {
		if strings.Contains(page, gone) {
			t.Errorf("page contains %q, but docs/ and test/ must be left out", gone)
		}
	}
	if strings.Count(page, "n_checks__[") != 1 {
		t.Error("collapsed folders are one box")
	}
	if strings.Contains(page, "n_smoke") || strings.Contains(page, "n_b -->") {
		t.Error("test-only packages and arrows between single packages are left out of the diagram")
	}
}

func TestRunWritesAndChecks(t *testing.T) {
	useFixtureLayers(t)
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

	// A package that uses a layer above it fails, even when writing.
	os.WriteFile(filepath.Join(root, "c", "c.go"), []byte("// Package c is new.\npackage c\n\nimport _ \"example.com/demo/b\"\n"), 0o644)
	if code := run(root, false); code != 1 {
		t.Error("a package using a layer above it must fail")
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

// Every Go file (except test fixtures) must start with a short
// description, so the file map on the architecture page explains the code.
// Other files, like LICENSE or configs, are optional.
func TestEveryFileDescribed(t *testing.T) {
	r, err := Scan("../..")
	if err != nil {
		t.Fatal(err)
	}
	var missing []string
	for _, f := range r.Files {
		if _, fixture := testdataDir(f); !fixture && strings.HasSuffix(f, ".go") && r.Notes[f] == "" {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		t.Errorf("these Go files need a one-line description at the top (see docs/contributing.md):\n  %s",
			strings.Join(missing, "\n  "))
	}
}

// hasLine reports whether one line of page contains both start and end
// (or has no note at all when end is "").
func hasLine(page, start, end string) bool {
	for _, line := range strings.Split(page, "\n") {
		if !strings.Contains(line, start) {
			continue
		}
		if end == "" && !strings.Contains(line, `class="note"`) || end != "" && strings.Contains(line, end) {
			return true
		}
	}
	return false
}

func TestNoteHTML(t *testing.T) {
	for in, want := range map[string]string{
		"":                       "",
		"Plain note.":            ` <span class="note">Plain note.</span>`,
		"Builds `docs/` <fast>.": ` <span class="note">Builds <code>docs/</code> &lt;fast&gt;.</span>`,
	} {
		if got := noteHTML(in); got != want {
			t.Errorf("noteHTML(%q) = %q, want %q", in, got, want)
		}
	}
}
