// Compares CLI reports for HTTP, Telnet and FTP over TLS with their saved JSON.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/nuqs-org/nuqs/checks/ftpanon"
	"github.com/nuqs-org/nuqs/checks/httpheaders"
	"github.com/nuqs-org/nuqs/checks/telnet"
	"github.com/nuqs-org/nuqs/internal/check"
)

var httpLabPort = flag.Int("http-lab-port", 0, "loopback port of the Compose HTTP fixture (default: in-process fixture)")

func TestHTTPReport(t *testing.T) {
	if *httpLabPort < 0 || *httpLabPort > 65535 {
		t.Fatal("invalid HTTP fixture port")
	}
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(*httpLabPort))
	if *httpLabPort == 0 {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(server.Close)
		address = server.Listener.Addr().String()
	}

	// The inventory and report always use port 8080. Route the request to the
	// disposable fixture's actual port so parallel runs can share a machine.
	transport := &http.Transport{DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	checkFixtureReport(t, "http", httpheaders.Check{Client: client})
}

func TestTelnetReport(t *testing.T) {
	checkFixtureReport(t, "telnet", telnet.Check{})
}

func TestFTPSReport(t *testing.T) {
	checkFixtureReport(t, "ftps", ftpanon.Check{})
}

func checkFixtureReport(t *testing.T, name string, selected check.Check) {
	t.Helper()
	discovery, err := os.ReadFile(filepath.Join("testdata", name+".xml"))
	if err != nil {
		t.Fatal(err)
	}
	app, stdout, stderr := newApp(t, &fakeRunner{out: discovery})
	app.Checks = func() []check.Check { return []check.Check{selected} }
	if code := app.Run(t.Context(), []string{"scan", "--json", "127.0.0.1"}); code != ExitOK {
		t.Fatalf("CLI exited %d: %s", code, stderr)
	}

	// Keep every JSON field, including future additions. Only the wall clock
	// changes between runs. Validate both timestamps before normalizing them.
	var report map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	var started, finished time.Time
	if err := json.Unmarshal(report["started"], &started); err != nil {
		t.Fatalf("started: %v", err)
	}
	if err := json.Unmarshal(report["finished"], &finished); err != nil {
		t.Fatalf("finished: %v", err)
	}
	if started.IsZero() || finished.Before(started) {
		t.Fatalf("invalid report interval: %v to %v", started, finished)
	}
	report["started"] = json.RawMessage(`"2000-01-01T00:00:00Z"`)
	report["finished"] = json.RawMessage(`"2000-01-01T00:00:01Z"`)
	got, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	golden := filepath.Join("testdata", name+"-report.json")
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, got) {
		actual := filepath.Join(t.TempDir(), "actual.json")
		if err := os.WriteFile(actual, got, 0600); err != nil {
			t.Fatal(err)
		}
		diff, err := exec.Command("git", "diff", "--no-index", "--no-ext-diff", "--", golden, actual).CombinedOutput()
		t.Fatalf("report differs from %s (%v)\n%s", golden, err, diff)
	}
}
