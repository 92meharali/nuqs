package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// outFile is where the page is written, relative to the repo root.
const outFile = "docs/architecture.md"

// collapse lists folders whose sub-packages are drawn as one box, so the
// diagram stays readable when we have 50 checks instead of 3.
var collapse = []string{"checks"}

// folderNotes describes folders that have no Go code of their own.
// Folders with Go code use their package doc comment instead.
var folderNotes = map[string]string{
	"cmd":               "Programs you can run. Each folder here is one binary.",
	"internal":          "The core of Nuqs. Only code in this repo can import these.",
	"tools":             "Helper programs for developing Nuqs. Not part of the scanner.",
	"scripts":           "Small helper scripts used by CI.",
	".github":           "GitHub settings.",
	".github/workflows": "CI jobs. They run on every push and pull request.",
}

// node returns the diagram box a package belongs to.
func node(dir string) string {
	for _, c := range collapse {
		if strings.HasPrefix(dir, c+"/") {
			return c + "/*"
		}
	}
	return dir
}

func nodeID(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return "n_" + b.String()
}

func topFolder(dir string) string {
	top, _, _ := strings.Cut(dir, "/")
	return top
}

// Render builds the whole architecture page.
func Render(r *Repo) string {
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }

	w("# Architecture\n\n")
	w("!!! note \"This page is generated\"\n")
	w("    `tools/archdiagram` builds it from the code on every push, so it always matches what's in the repo. Don't edit it by hand. To change a description, change the package's doc comment.\n\n")

	w("## How the packages connect\n\n")
	w("An arrow means \"uses\". For example `internal/cli --> internal/engine` means the CLI calls the engine.")
	if hasTestOnly(r) {
		w(" Dotted arrows come from test-only packages.")
	}
	w("\n\n")
	renderDiagram(&b, r)

	w("\n## Packages\n\n")
	w("| Package | What it does | Uses |\n|---|---|---|\n")
	for _, p := range r.Packages {
		uses := "nothing else in Nuqs"
		if len(p.Imports) > 0 {
			uses = "`" + strings.Join(p.Imports, "`, `") + "`"
		}
		name := "`" + p.Dir + "`"
		if p.TestOnly {
			name += " (tests)"
		}
		w("| %s | %s | %s |\n", name, orDash(p.Summary), uses)
	}

	w("\n## What's where\n\n")
	w("Every file in the repo, as git sees it, except the `docs/` and `test/` folders.\n\n")
	w("```text\n%s```\n", tree(r))
	return b.String()
}

func renderDiagram(b *strings.Builder, r *Repo) {
	w := func(format string, a ...any) { fmt.Fprintf(b, format, a...) }

	// Work out the boxes and the arrows between them.
	labels := map[string]string{}
	members := map[string][]string{}
	testOnly := map[string]bool{}
	type edge struct {
		from, to string
		dotted   bool
	}
	var edges []edge
	for _, p := range r.Packages {
		n := node(p.Dir)
		members[n] = append(members[n], strings.TrimPrefix(p.Dir, strings.TrimSuffix(n, "*")))
		testOnly[n] = p.TestOnly
		for _, imp := range p.Imports {
			to := node(imp)
			if to != n {
				edges = append(edges, edge{n, to, p.TestOnly})
			}
		}
	}
	for n, m := range members {
		labels[n] = n
		if strings.HasSuffix(n, "/*") {
			slices.Sort(m)
			labels[n] = fmt.Sprintf("%s<br/><small>%s</small>", n, strings.Join(m, ", "))
		}
	}
	slices.SortFunc(edges, func(a, c edge) int {
		return strings.Compare(a.from+" "+a.to, c.from+" "+c.to)
	})
	edges = slices.Compact(edges)

	// Group boxes by top folder so related things sit together.
	groups := map[string][]string{}
	for n := range labels {
		groups[topFolder(n)] = append(groups[topFolder(n)], n)
	}
	var tops []string
	for t := range groups {
		tops = append(tops, t)
	}
	slices.Sort(tops)

	w("```mermaid\nflowchart LR\n")
	for _, t := range tops {
		nodes := groups[t]
		slices.Sort(nodes)
		w("  subgraph %s [\"%s\"]\n", nodeID("g_"+t), t)
		for _, n := range nodes {
			w("    %s[\"%s\"]\n", nodeID(n), labels[n])
		}
		w("  end\n")
	}
	for _, e := range edges {
		arrow := "-->"
		if e.dotted {
			arrow = "-.->"
		}
		w("  %s %s %s\n", nodeID(e.from), arrow, nodeID(e.to))
	}
	w("```\n")
}

func hasTestOnly(r *Repo) bool {
	return slices.ContainsFunc(r.Packages, func(p Package) bool { return p.TestOnly })
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return strings.ReplaceAll(s, "|", "\\|")
}

// tree draws the folders and files like the `tree` command, with a short
// note after each folder that has one.
func tree(r *Repo) string {
	notes := map[string]string{}
	for k, v := range folderNotes {
		notes[k] = v
	}
	for _, p := range r.Packages {
		if p.Summary != "" {
			notes[p.Dir] = p.Summary
		}
	}
	for _, f := range r.Files {
		for d := filepath.Dir(f); d != "."; d = filepath.Dir(d) {
			if filepath.Base(d) == "testdata" && notes[d] == "" {
				notes[d] = "Sample files used by the tests next to it."
			}
		}
	}

	// Build a nested structure from the flat file list.
	type dir struct {
		dirs  map[string]*dir
		files []string
	}
	newDir := func() *dir { return &dir{dirs: map[string]*dir{}} }
	root := newDir()
	for _, f := range r.Files {
		parts := strings.Split(f, "/")
		d := root
		for _, part := range parts[:len(parts)-1] {
			if d.dirs[part] == nil {
				d.dirs[part] = newDir()
			}
			d = d.dirs[part]
		}
		d.files = append(d.files, parts[len(parts)-1])
	}

	var b strings.Builder
	var walk func(d *dir, path, indent string)
	walk = func(d *dir, path, indent string) {
		var names []string
		for n := range d.dirs {
			names = append(names, n)
		}
		slices.Sort(names)
		total := len(names) + len(d.files)
		i := 0
		for _, n := range names {
			i++
			branch, next := "├── ", "│   "
			if i == total {
				branch, next = "└── ", "    "
			}
			full := strings.TrimPrefix(path+"/"+n, "/")
			line := indent + branch + n + "/"
			if note := notes[full]; note != "" {
				line += pad(line) + "# " + note
			}
			b.WriteString(line + "\n")
			walk(d.dirs[n], full, indent+next)
		}
		for _, f := range d.files {
			i++
			branch := "├── "
			if i == total {
				branch = "└── "
			}
			b.WriteString(indent + branch + f + "\n")
		}
	}
	b.WriteString(".\n")
	walk(root, "", "")
	return b.String()
}

// pad lines notes up in one column so the tree is easy to scan.
func pad(line string) string {
	const col = 34
	n := len([]rune(line))
	if n >= col {
		return "  "
	}
	return strings.Repeat(" ", col-n)
}
