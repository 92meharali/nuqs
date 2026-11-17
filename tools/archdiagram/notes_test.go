// Tests reading the one-line description at the top of each kind of file.

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileNote(t *testing.T) {
	cases := []struct {
		name, content, want string
	}{
		{"a.go", "// Does one thing. More detail.\n\npackage a\n", "Does one thing."},
		{"b.go", "//go:build integration\n\n// Runs slow tests.\n\npackage b\n", "Runs slow tests."},
		{"c.go", "// Package c is the c package. Long text.\npackage c\n", "The c package."},
		{"d.go", "package d\n", ""},
		{"e.go", "// File note.\n\n// Package e does e things.\npackage e\n", "File note."},
		{"bad.go", "this is not go", ""},
		{"run.sh", "#!/usr/bin/env bash\n# Builds the thing.\n# More.\nset -e\n", "Builds the thing."},
		{"ci.yml", "\n# Runs CI.\n\nname: CI\n", "Runs CI."},
		{"x.yml", "name: no comment\n# too late\n", ""},
		{".gitignore", "# Files to ignore.\n\n/bin\n", "Files to ignore."},
		{"go.mod", "// The module.\n\nmodule x\n", "The module."},
		{"README.md", "<!-- Front page. -->\n\n# Title\n", "Front page."},
		{"doc.md", "# Title\n", ""},
		{"scan.xml", "<?xml version=\"1.0\"?>\n<!DOCTYPE nmaprun>\n<!-- A sample scan. -->\n<nmaprun/>\n", "A sample scan."},
		{"open.xml", "<!-- never closed\n<a/>\n", ""},
	}
	dir := t.TempDir()
	for _, c := range cases {
		if err := os.WriteFile(filepath.Join(dir, c.name), []byte(c.content), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := fileNote(dir, c.name); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
	if got := fileNote(dir, "missing.yml"); got != "" {
		t.Errorf("a missing file has no note, got %q", got)
	}
}
