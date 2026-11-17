// Reads the repo: its packages, what they import, and its files.

package main

import (
	"bufio"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// Package is one Go package (one folder) in the repo.
type Package struct {
	Dir      string   // folder relative to the repo root, like "internal/engine"
	Summary  string   // first sentence of the package doc comment
	Imports  []string // other packages from this repo that it imports (as folders)
	TestOnly bool     // the folder only has _test.go files
}

// Repo is everything the generator learned about the repo.
type Repo struct {
	Module   string
	Packages []Package         // sorted by Dir
	Files    []string          // every file shown in the tree, sorted, relative to root
	Notes    map[string]string // file path -> the description at the top of that file
}

// Scan reads the repo at root and works out its packages and files.
//
// Files come from git (tracked files plus new ones that aren't ignored), so
// the page shows exactly what's in the repo and never local junk like build
// output. Outside a git repo it falls back to walking the folders.
func Scan(root string) (*Repo, error) {
	module, err := readModule(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil, err
	}
	files, err := gitFiles(root)
	if err != nil {
		files, err = walkFiles(root)
		if err != nil {
			return nil, err
		}
	}
	r := &Repo{Module: module, Notes: map[string]string{}}
	pkgs := map[string]*Package{}
	for _, rel := range files {
		if excluded(rel) {
			continue
		}
		r.Files = append(r.Files, rel)
		if note := fileNote(root, rel); note != "" {
			r.Notes[rel] = note
		}
		// Go files under testdata are test fixtures, not packages.
		if strings.HasSuffix(rel, ".go") && !inTestdata(rel) {
			if err := addGoFile(pkgs, module, root, rel); err != nil {
				return nil, err
			}
		}
	}

	for _, p := range pkgs {
		slices.Sort(p.Imports)
		p.Imports = slices.Compact(p.Imports)
		r.Packages = append(r.Packages, *p)
	}
	slices.SortFunc(r.Packages, func(a, b Package) int { return strings.Compare(a.Dir, b.Dir) })
	slices.Sort(r.Files)
	return r, nil
}

// addGoFile records the package doc and repo-internal imports of one file.
// Test files only count when the folder has no normal files.
func addGoFile(pkgs map[string]*Package, module, root, rel string) error {
	f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, rel), nil, parser.ImportsOnly|parser.ParseComments)
	if err != nil {
		return fmt.Errorf("%s: %w", rel, err)
	}
	dir := filepath.ToSlash(filepath.Dir(rel))
	isTest := strings.HasSuffix(rel, "_test.go")

	p, ok := pkgs[dir]
	if !ok {
		p = &Package{Dir: dir, TestOnly: true}
		pkgs[dir] = p
	}
	if !isTest && p.TestOnly {
		// First normal file: forget anything we took from test files.
		p.TestOnly, p.Summary, p.Imports = false, "", nil
	}
	if isTest && !p.TestOnly {
		return nil
	}
	if p.Summary == "" && f.Doc != nil {
		p.Summary = summary(f.Name.Name, f.Doc.Text())
	}
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if path == module || strings.HasPrefix(path, module+"/") {
			p.Imports = append(p.Imports, strings.TrimPrefix(strings.TrimPrefix(path, module), "/"))
		}
	}
	return nil
}

// skipFolders are left out of the page completely. docs/ and test/ don't
// help someone find their way around the code, and site/ is generated.
var skipFolders = []string{"docs", "test", "site"}

// excluded reports whether a file is inside one of skipFolders.
func excluded(rel string) bool {
	for _, d := range skipFolders {
		if strings.HasPrefix(rel, d+"/") {
			return true
		}
	}
	return false
}

func inTestdata(rel string) bool {
	return strings.HasPrefix(rel, "testdata/") || strings.Contains(rel, "/testdata/")
}

// gitFiles lists tracked files plus new files that aren't ignored.
func gitFiles(root string) ([]string, error) {
	out, err := exec.Command("git", "-C", root, "ls-files", "--cached", "--others", "--exclude-standard").Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, f := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		// Skip files that are tracked but were deleted and not committed yet.
		if f != "" {
			if _, err := os.Stat(filepath.Join(root, f)); err == nil {
				files = append(files, f)
			}
		}
	}
	slices.Sort(files)
	return slices.Compact(files), nil
}

// walkFiles is the fallback when root isn't a git repo. It skips hidden
// folders (except .github) and common build or dependency folders.
func walkFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		name := d.Name()
		if d.IsDir() {
			hidden := strings.HasPrefix(name, ".") && name != ".github" && rel != "."
			if hidden || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if name != ".DS_Store" {
			files = append(files, rel)
		}
		return nil
	})
	return files, err
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

// summary returns the first sentence of a doc comment, on one line.
// "Package engine runs a scan." becomes "Runs a scan." because the
// package name is already shown next to it.
func summary(pkgName, doc string) string {
	doc = strings.Join(strings.Fields(doc), " ")
	if i := strings.Index(doc, ". "); i >= 0 {
		doc = doc[:i+1]
	}
	prefixes := []string{"Package " + pkgName + " "}
	if pkgName == "main" { // "Command nuqs is ..." names the binary, not the package
		if f := strings.Fields(doc); len(f) > 2 && f[0] == "Command" {
			prefixes = append(prefixes, "Command "+f[1]+" ")
		}
	}
	for _, prefix := range prefixes {
		if rest, ok := strings.CutPrefix(doc, prefix); ok && rest != "" {
			rest = strings.TrimPrefix(rest, "is ") // "Package cli is the CLI." reads better as "The CLI."
			return strings.ToUpper(rest[:1]) + rest[1:]
		}
	}
	return doc
}
