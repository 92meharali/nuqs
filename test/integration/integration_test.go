//go:build integration

// Package integration runs the whole scan with the real Nmap binary.
//
// It starts two small local servers with known problems, scans them and
// checks we report exactly those problems. Run it with:
//
//	go test -tags integration ./test/integration
//
// Nmap must be installed. CI installs it for this job.
package integration

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/PremadeS/nuqs/checks"
	"github.com/PremadeS/nuqs/internal/cli"
	"github.com/PremadeS/nuqs/internal/engine"
	"github.com/PremadeS/nuqs/internal/nmap"
)

func TestFullScan(t *testing.T) {
	if _, err := exec.LookPath("nmap"); err != nil {
		t.Skip("nmap is not installed")
	}

	// A web server with no security headers.
	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("<html>hello</html>"))
	}))
	defer web.Close()
	_, webPort, _ := net.SplitHostPort(web.Listener.Addr().String())

	// An FTP server that lets anyone in.
	ftp, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ftp.Close()
	go serveAnonFTP(ftp)
	_, ftpPort, _ := net.SplitHostPort(ftp.Addr().String())

	var out, errOut bytes.Buffer
	app := &cli.App{
		Version:   "integration",
		Stdout:    &out,
		Stderr:    &errOut,
		NewRunner: func(path string) nmap.Runner { return nmap.ExecRunner{Path: path} },
		Checks:    checks.All,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	code := app.Run(ctx, []string{"scan", "--json", "-Pn", "-p", webPort + "," + ftpPort, "127.0.0.1"})
	if code != cli.ExitOK {
		t.Fatalf("exit %d\nstderr:\n%s", code, errOut.String())
	}

	var rep engine.Report
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, out.String())
	}
	got := map[string]bool{}
	for _, f := range rep.Findings {
		got[f.CheckID] = true
	}
	for _, want := range []string{"http-security-headers", "ftp-anonymous-login"} {
		if !got[want] {
			t.Errorf("expected a %s finding, got findings %+v\nhosts %+v\nerrors %+v", want, rep.Findings, rep.Hosts, rep.Errors)
		}
	}
	if got["telnet-open"] {
		t.Error("there is no telnet server, so no telnet finding")
	}
}

// serveAnonFTP answers like vsftpd with anonymous login turned on.
func serveAnonFTP(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		go func(c net.Conn) {
			defer c.Close()
			c.SetDeadline(time.Now().Add(30 * time.Second))
			c.Write([]byte("220 (vsFTPd 3.0.3)\r\n"))
			sc := bufio.NewScanner(c)
			for sc.Scan() {
				cmd := strings.ToUpper(strings.Fields(sc.Text() + " x")[0])
				switch cmd {
				case "USER":
					c.Write([]byte("331 Please specify the password.\r\n"))
				case "PASS":
					c.Write([]byte("230 Login successful.\r\n"))
				case "QUIT":
					c.Write([]byte("221 Goodbye.\r\n"))
					return
				default:
					c.Write([]byte("500 Unknown command.\r\n"))
				}
			}
		}(conn)
	}
}
