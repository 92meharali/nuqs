// Package output includes report exports for browsers, PDF readers and spreadsheets.
package output

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"html/template"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/nuqs-org/nuqs/internal/engine"
	"github.com/nuqs-org/nuqs/internal/model"
)

var reportHTML = template.Must(template.New("report").Funcs(template.FuncMap{"join": strings.Join}).Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Nuqs scan report</title><style>
:root{color-scheme:light;font:14px/1.5 -apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;color:#1d1d1f;background:#f5f5f7}
body{max-width:1000px;margin:0 auto;padding:40px 24px 64px}header{border-bottom:2px solid #111;padding-bottom:18px;margin-bottom:24px}
h1{font-size:30px;margin:0 0 4px}h2{font-size:18px;margin:28px 0 10px}p{margin:4px 0}.muted{color:#666}.summary{display:grid;grid-template-columns:repeat(3,1fr);gap:12px}
.summary div,.finding,.notice{background:#fff;border:1px solid #ddd;border-radius:10px;padding:14px}.summary strong{display:block;font-size:24px}
table{width:100%;border-collapse:collapse;background:#fff}th,td{text-align:left;vertical-align:top;padding:9px;border-bottom:1px solid #ddd}th{background:#eee}
.finding{margin:10px 0}.finding h3{margin:0 0 6px;font-size:15px}.severity{font-weight:700;text-transform:uppercase}.critical,.high{color:#b42318}.medium{color:#8a5700}.low{color:#175cd3}
code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;overflow-wrap:anywhere}@media print{body{padding:0}.finding,.summary div{break-inside:avoid}}
</style></head><body><header><h1>Nuqs scan report</h1><p class="muted">Targets: {{join .Report.Targets ", "}}</p><p class="muted">{{.Started}} &ndash; {{.Finished}} ({{.Duration}})</p></header>
<main><section class="summary"><div><strong>{{len .Report.Hosts}}</strong>hosts</div><div><strong>{{.Services}}</strong>open services</div><div><strong>{{len .Report.Findings}}</strong>findings</div></section>
<h2>Findings</h2>{{if .Findings}}{{range .Findings}}<article class="finding"><h3><span class="severity {{.Severity}}">{{.Severity}}</span> &middot; {{.Title}}</h3><p><code>{{.Host}}:{{.Port}}/{{.Protocol}}</code> &middot; {{.Confidence}} &middot; <code>{{.CheckID}}</code></p><p><strong>Evidence:</strong> {{.Evidence}}</p><p><strong>Fix:</strong> {{.Fix}}</p></article>{{end}}{{else}}<p class="notice">No findings.</p>{{end}}
<h2>Hosts and services</h2>{{if .Report.Hosts}}<table><thead><tr><th>Host</th><th>Hostnames</th><th>Open services</th></tr></thead><tbody>{{range .Report.Hosts}}<tr><td><code>{{.Address}}</code></td><td>{{join .Hostnames ", "}}</td><td>{{range .Services}}<div><code>{{.Port}}/{{.Protocol}}</code> {{.Name}} {{.Product}} {{.Version}}</div>{{end}}</td></tr>{{end}}</tbody></table>{{else}}<p class="notice">No hosts answered.</p>{{end}}
{{if .Report.Errors}}<h2>Checks that could not finish</h2><table><thead><tr><th>Check</th><th>Host</th><th>Error</th></tr></thead><tbody>{{range .Report.Errors}}<tr><td><code>{{.CheckID}}</code></td><td><code>{{.Host}}:{{.Port}}</code></td><td>{{.Error}}</td></tr>{{end}}</tbody></table>{{end}}</main></body></html>`))

type exportFinding struct {
	model.Finding
	Severity   string
	Confidence model.Confidence
}

type htmlData struct {
	Report   *engine.Report
	Findings []exportFinding
	Started  string
	Finished string
	Duration string
	Services int
}

// HTML writes a self-contained, printable scan report.
func HTML(w io.Writer, r *engine.Report) error {
	data := htmlData{
		Report: r, Started: r.Started.Format("2006-01-02 15:04:05 MST"), Finished: r.Finished.Format("2006-01-02 15:04:05 MST"),
		Duration: r.Finished.Sub(r.Started).Round(100 * time.Millisecond).String(), Services: countServices(r),
	}
	for _, finding := range r.Findings {
		data.Findings = append(data.Findings, exportFinding{Finding: finding, Severity: finding.Severity.String(), Confidence: confidence(finding)})
	}
	return reportHTML.Execute(w, data)
}

// CSV writes one spreadsheet row per finding. Text that spreadsheet programs
// could interpret as a formula is prefixed with an apostrophe.
func CSV(w io.Writer, r *engine.Report) error {
	c := csv.NewWriter(w)
	if err := c.Write([]string{"finding_id", "severity", "confidence", "host", "port", "protocol", "check_id", "title", "evidence", "fix"}); err != nil {
		return err
	}
	for _, f := range r.Findings {
		row := []string{f.ID, f.Severity.String(), string(confidence(f)), f.Host, strconv.Itoa(f.Port), f.Protocol, f.CheckID, f.Title, f.Evidence, f.Fix}
		for i := range row {
			row[i] = spreadsheetText(row[i])
		}
		if err := c.Write(row); err != nil {
			return err
		}
	}
	c.Flush()
	return c.Error()
}

func spreadsheetText(s string) string {
	trimmed := strings.TrimLeftFunc(s, unicode.IsSpace)
	if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + s
	}
	return s
}

// PDF writes a compact, dependency-free PDF report using the built-in
// Helvetica font. Non-ASCII characters are replaced because standard PDF
// fonts do not carry a Unicode character map.
func PDF(w io.Writer, r *engine.Report) error {
	lines := []string{
		"Nuqs scan report", "",
		"Targets: " + strings.Join(r.Targets, ", "),
		fmt.Sprintf("Started: %s", r.Started.Format("2006-01-02 15:04:05 MST")),
		fmt.Sprintf("Duration: %s", r.Finished.Sub(r.Started).Round(100*time.Millisecond)),
		fmt.Sprintf("Hosts: %d    Open services: %d    Findings: %d", len(r.Hosts), countServices(r), len(r.Findings)), "", "Findings",
	}
	if len(r.Findings) == 0 {
		lines = append(lines, "No findings.")
	}
	for _, f := range r.Findings {
		lines = append(lines,
			fmt.Sprintf("[%s] %s", strings.ToUpper(f.Severity.String()), f.Title),
			fmt.Sprintf("%s:%d/%s | %s | %s", f.Host, f.Port, f.Protocol, confidence(f), f.CheckID),
			"Evidence: "+f.Evidence, "Fix: "+f.Fix, "")
	}
	lines = append(lines, "Hosts and services")
	if len(r.Hosts) == 0 {
		lines = append(lines, "No hosts answered.")
	}
	for _, host := range r.Hosts {
		name := host.Address
		if len(host.Hostnames) > 0 {
			name += " (" + strings.Join(host.Hostnames, ", ") + ")"
		}
		lines = append(lines, name)
		for _, service := range host.Services {
			lines = append(lines, fmt.Sprintf("  %d/%s  %s %s %s", service.Port, service.Protocol, service.Name, service.Product, service.Version))
		}
	}
	if len(r.Errors) > 0 {
		lines = append(lines, "Checks that could not finish")
		for _, e := range r.Errors {
			lines = append(lines, fmt.Sprintf("%s on %s:%d: %s", e.CheckID, e.Host, e.Port, firstLine(e.Error)))
		}
	}
	var wrapped []string
	for _, line := range lines {
		wrapped = append(wrapped, wrapPDFLine(line, 92)...)
	}
	return writePDF(w, wrapped)
}

func wrapPDFLine(s string, width int) []string {
	s = ascii(s)
	if len(s) <= width {
		return []string{s}
	}
	var out []string
	for len(s) > width {
		cut := strings.LastIndexByte(s[:width+1], ' ')
		if cut < width/2 {
			cut = width
		}
		out = append(out, strings.TrimSpace(s[:cut]))
		s = strings.TrimSpace(s[cut:])
	}
	return append(out, s)
}

func ascii(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 32 && r <= 126 {
			return r
		}
		return '?'
	}, s)
}

func pdfString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `(`, `\(`)
	return strings.ReplaceAll(s, `)`, `\)`)
}

func writePDF(w io.Writer, lines []string) error {
	const perPage = 48
	pages := (len(lines) + perPage - 1) / perPage
	if pages == 0 {
		pages = 1
	}
	objects := make([][]byte, 3+pages*2)
	objects[0] = []byte("<< /Type /Catalog /Pages 2 0 R >>")
	var kids strings.Builder
	for i := range pages {
		fmt.Fprintf(&kids, "%d 0 R ", 4+i*2)
	}
	objects[1] = []byte(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", kids.String(), pages))
	objects[2] = []byte("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	for i := range pages {
		start := i * perPage
		end := min(start+perPage, len(lines))
		var content strings.Builder
		content.WriteString("BT /F1 10 Tf 50 742 Td 14 TL\n")
		for _, line := range lines[start:end] {
			fmt.Fprintf(&content, "(%s) Tj T*\n", pdfString(line))
		}
		content.WriteString("ET")
		pageObj, streamObj := 4+i*2, 5+i*2
		objects[pageObj-1] = []byte(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", streamObj))
		objects[streamObj-1] = []byte(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", content.Len(), content.String()))
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	offsets := make([]int, len(objects)+1)
	for i, object := range objects {
		offsets[i+1] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	_, err := w.Write(b.Bytes())
	return err
}
