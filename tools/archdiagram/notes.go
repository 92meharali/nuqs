// Reads the one-line description at the top of each file.

package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// fileNote returns the description written at the top of a file, or "" if
// it has none. The rules are simple, so they're easy to follow:
//
//   - Go: a comment at the very top, with a blank line before "package"
//     (so Go doesn't mistake it for the package doc). A file that holds the
//     package doc and has no such line uses the package doc instead.
//   - YAML, shell, .gitignore, go.mod and similar: the first comment line.
//   - XML and Markdown: a <!-- comment --> at the top.
func fileNote(root, rel string) string {
	path := filepath.Join(root, rel)
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".go":
		return goFileNote(path)
	case ".xml", ".md", ".html":
		return htmlCommentNote(path)
	case ".mod":
		return lineCommentNote(path, "//")
	default:
		return lineCommentNote(path, "#")
	}
}

func goFileNote(path string) string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.PackageClauseOnly|parser.ParseComments)
	if err != nil {
		return ""
	}
	for _, g := range f.Comments {
		if g.Pos() >= f.Package {
			break
		}
		if g == f.Doc {
			continue
		}
		text := strings.TrimSpace(g.Text()) // Text() drops //go:build and other directives
		if text != "" {
			return summary(f.Name.Name, text)
		}
	}
	if f.Doc != nil {
		return summary(f.Name.Name, f.Doc.Text())
	}
	return ""
}

// lineCommentNote reads comment lines at the top of a file, skipping a
// shebang and blank lines, and returns their first sentence.
func lineCommentNote(path, marker string) string {
	lines := topLines(path)
	var parts []string
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if i == 0 && strings.HasPrefix(line, "#!") {
			continue
		}
		if line == "" {
			if len(parts) > 0 {
				break
			}
			continue
		}
		text, ok := strings.CutPrefix(line, marker)
		if !ok {
			break
		}
		parts = append(parts, strings.TrimSpace(text))
	}
	return summary("", strings.Join(parts, " "))
}

// htmlCommentNote reads a <!-- comment --> at the top, after an optional
// <?xml ...?> line or <!DOCTYPE>.
func htmlCommentNote(path string) string {
	text := strings.Join(topLines(path), "\n")
	for {
		text = strings.TrimSpace(text)
		if strings.HasPrefix(text, "<?") || strings.HasPrefix(text, "<!DOCTYPE") {
			if i := strings.Index(text, ">"); i >= 0 {
				text = text[i+1:]
				continue
			}
		}
		break
	}
	rest, ok := strings.CutPrefix(text, "<!--")
	if !ok {
		return ""
	}
	end := strings.Index(rest, "-->")
	if end < 0 {
		return ""
	}
	return summary("", rest[:end])
}

// topLines returns the first lines of a file. Descriptions live at the top,
// so we never read more than a few KB.
func topLines(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	buf := make([]byte, 4096)
	n, _ := f.Read(buf)
	return strings.Split(string(buf[:n]), "\n")
}
