// Tests TLS self-signed generation, network HTTPS serving, first-run setup code output and "nuqs user reset-password".

package cli

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServeSetupCodeAndAuthenticatedRoutes(t *testing.T) {
	dataDir := t.TempDir()
	r := &fakeRunner{}
	app, out, errOut := newApp(t, r)
	app.PBKDF2Iterations = 1000
	app.LookPath = func(file string) (string, error) { return "/usr/bin/" + file, nil }

	var servedHandler http.Handler
	app.ListenAndServe = func(_ context.Context, _ string, h http.Handler) error {
		servedHandler = h
		return nil
	}

	code := app.Run(context.Background(), []string{"serve", "--addr", "127.0.0.1:9123", "--data", dataDir})
	if code != ExitOK {
		t.Fatalf("exit %d, stderr:\n%s", code, errOut)
	}
	stdout := out.String()
	if !strings.Contains(stdout, "First-time setup required") || !strings.Contains(stdout, "http://127.0.0.1:9123/setup") {
		t.Fatalf("expected setup code instructions in stdout:\n%s", stdout)
	}
	setupCode := extractIndentedCode(t, stdout)

	// Before setup: /scans redirects to /setup, /api/status returns 401.
	rec := httptest.NewRecorder()
	servedHandler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://127.0.0.1:9123/scans", nil))
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/setup" {
		t.Fatalf("uninitialized /scans: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	rec = httptest.NewRecorder()
	servedHandler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://127.0.0.1:9123/api/status", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("uninitialized /api/status: %d", rec.Code)
	}

	// Complete setup through the wired handler.
	form := url.Values{"code": {setupCode}, "username": {"admin"}, "password": {"admin-secret-123"}, "confirm": {"admin-secret-123"}}
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:9123/setup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://127.0.0.1:9123")
	rec = httptest.NewRecorder()
	servedHandler.ServeHTTP(rec, req)
	setupCookies := resultCookies(rec)
	if rec.Code != http.StatusSeeOther || len(setupCookies) == 0 {
		t.Fatalf("POST /setup: %d %s", rec.Code, rec.Body.String())
	}
	sessionCookie := setupCookies[0]

	// Authenticated requests reach all routes.
	for _, path := range []string{"/scans", "/scans/new", "/checks", "/users", "/api/status", "/api/checks", "/api/scans", "/api/users", "/partials/checks", "/static/nuqs.css"} {
		req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:9123"+path, nil)
		req.AddCookie(sessionCookie)
		rec := httptest.NewRecorder()
		servedHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("authenticated GET %s: got %d, want 200", path, rec.Code)
		}
	}

	// Restarting nuqs serve with existing users must NOT print a setup code.
	app2, out2, errOut2 := newApp(t, r)
	app2.PBKDF2Iterations = 1000
	app2.LookPath = func(file string) (string, error) { return "/usr/bin/" + file, nil }
	app2.ListenAndServe = func(context.Context, string, http.Handler) error { return nil }
	if code := app2.Run(context.Background(), []string{"serve", "--addr", "127.0.0.1:9123", "--data", dataDir}); code != ExitOK {
		t.Fatalf("restart serve: %d %s", code, errOut2)
	}
	if strings.Contains(out2.String(), "setup code") {
		t.Fatalf("should not print setup code after setup:\n%s", out2.String())
	}

	// Run "nuqs user reset-password" from CLI and verify it revokes the running server's session and prints a reset code.
	cliApp, cliOut, cliErr := newApp(t, r)
	cliApp.PBKDF2Iterations = 1000
	if code := cliApp.Run(context.Background(), []string{"user", "reset-password", "--data", dataDir, "admin"}); code != ExitOK {
		t.Fatalf("user reset-password: %d %s", code, cliErr)
	}
	resetCode := extractIndentedCode(t, cliOut.String())
	if resetCode == "" {
		t.Fatalf("missing reset code:\n%s", cliOut.String())
	}
	// Previous session is immediately revoked on the running server handler.
	req = httptest.NewRequest(http.MethodGet, "http://127.0.0.1:9123/api/status", nil)
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	servedHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("revoked session should fail immediately, got %d", rec.Code)
	}
	// Reset password completes via /reset-password using the CLI code.
	resetForm := url.Values{"username": {"admin"}, "code": {resetCode}, "password": {"new-password-456"}, "confirm": {"new-password-456"}}
	req = httptest.NewRequest(http.MethodPost, "http://127.0.0.1:9123/reset-password", strings.NewReader(resetForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://127.0.0.1:9123")
	rec = httptest.NewRecorder()
	servedHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login?notice=password-reset" {
		t.Fatalf("POST /reset-password: %d %q %s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
}

func TestServeHTTPSNetworkModeAndSelfSignedCert(t *testing.T) {
	dataDir := t.TempDir()
	app, out, errOut := newApp(t, &fakeRunner{})
	app.PBKDF2Iterations = 1000
	app.LookPath = func(file string) (string, error) { return "/usr/bin/" + file, nil }

	var gotAddr, gotCert, gotKey string
	var gotHandler http.Handler
	app.ListenAndServeTLS = func(_ context.Context, addr, certFile, keyFile string, h http.Handler) error {
		gotAddr, gotCert, gotKey, gotHandler = addr, certFile, keyFile, h
		return nil
	}

	code := app.Run(context.Background(), []string{"serve", "--addr", "0.0.0.0:9443", "--data", dataDir, "--tls-self-signed"})
	if code != ExitOK {
		t.Fatalf("serve HTTPS exit %d: %s", code, errOut)
	}
	if gotAddr != "0.0.0.0:9443" || gotCert == "" || gotKey == "" || !strings.Contains(out.String(), "https://127.0.0.1:9443") || !strings.Contains(out.String(), "0.0.0.0:9443") {
		t.Fatalf("unexpected HTTPS bind: addr=%q cert=%q key=%q out=%s", gotAddr, gotCert, gotKey, out)
	}
	cfg, err := BuildTLSConfig(gotCert, gotKey)
	if err != nil || cfg.MinVersion != tls.VersionTLS12 || len(cfg.Certificates) != 1 {
		t.Fatalf("BuildTLSConfig: %+v %v", cfg, err)
	}
	leaf, err := x509.ParseCertificate(cfg.Certificates[0].Certificate[0])
	if err != nil || len(leaf.IPAddresses) == 0 || len(leaf.DNSNames) == 0 {
		t.Fatalf("self-signed leaf cert SANs missing: %v", err)
	}
	// Reusing existing self-signed cert leaves the file untouched.
	infoBefore, _ := os.Stat(gotCert)
	cert2, key2, err := EnsureSelfSignedCert(dataDir, "0.0.0.0:9443")
	if err != nil || cert2 != gotCert || key2 != gotKey {
		t.Fatalf("reuse cert: %q %q %v", cert2, key2, err)
	}
	infoAfter, _ := os.Stat(cert2)
	if !infoAfter.ModTime().Equal(infoBefore.ModTime()) {
		t.Error("expected existing valid cert to be reused")
	}

	// Network HTTPS mode accepts LAN Host headers (not blocked by LocalOnly) and sets HSTS.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "https://192.168.1.50:9443/setup", nil)
	req.TLS = &tls.ConnectionState{}
	gotHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Header().Get("Strict-Transport-Security") == "" {
		t.Fatalf("LAN HTTPS /setup: code=%d hsts=%q", rec.Code, rec.Header().Get("Strict-Transport-Security"))
	}

	// Bad TLS flag combinations are rejected.
	for _, bad := range [][]string{
		{"serve", "--tls-cert", gotCert},
		{"serve", "--tls-key", gotKey},
		{"serve", "--tls-self-signed", "--tls-cert", gotCert, "--tls-key", gotKey},
		{"serve", "--addr", "0.0.0.0:9443"},
	} {
		a2, _, _ := newApp(t, &fakeRunner{})
		if c := a2.Run(context.Background(), bad); c != ExitUsage {
			t.Errorf("%v: exit %d, want %d", bad, c, ExitUsage)
		}
	}
}

func TestUserCommandUsageAndErrors(t *testing.T) {
	dataDir := t.TempDir()
	for _, c := range []struct {
		args []string
		want int
	}{
		{[]string{"user"}, ExitUsage},
		{[]string{"user", "-h"}, ExitOK},
		{[]string{"user", "bogus"}, ExitUsage},
		{[]string{"user", "reset-password", "-h"}, ExitOK},
		{[]string{"user", "reset-password"}, ExitUsage},
		{[]string{"user", "reset-password", "--data", dataDir, "nobody"}, ExitError},
	} {
		app, _, _ := newApp(t, &fakeRunner{})
		if got := app.Run(context.Background(), c.args); got != c.want {
			t.Errorf("%v: exit %d, want %d", c.args, got, c.want)
		}
	}
}

func extractIndentedCode(t *testing.T, output string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "  ") {
			return strings.TrimSpace(line)
		}
	}
	t.Fatalf("could not find indented one-time code in:\n%s", output)
	return ""
}

func resultCookies(w *httptest.ResponseRecorder) []*http.Cookie {
	resp := w.Result()
	defer resp.Body.Close()
	return resp.Cookies()
}

func TestTLSAndAddressHelpers(t *testing.T) {
	// checkLoopback
	for _, tc := range []struct {
		addr    string
		wantErr bool
	}{
		{"127.0.0.1:9002", false},
		{"localhost:9002", false},
		{"[::1]:9002", false},
		{"0.0.0.0:9002", true},
		{"192.168.1.1:9002", true},
		{"invalid-addr", true},
	} {
		err := checkLoopback(tc.addr)
		if (err != nil) != tc.wantErr {
			t.Errorf("checkLoopback(%q) = %v, wantErr %v", tc.addr, err, tc.wantErr)
		}
	}

	// CheckServeAddr
	for _, tc := range []struct {
		addr    string
		wantErr bool
	}{
		{"127.0.0.1:9002", false},
		{"0.0.0.0:443", false},
		{"127.0.0.1", true},
		{"127.0.0.1:abc", true},
		{"127.0.0.1:99999", true},
		{"127.0.0.1:-1", true},
	} {
		_, err := CheckServeAddr(tc.addr)
		if (err != nil) != tc.wantErr {
			t.Errorf("CheckServeAddr(%q) err=%v, wantErr=%v", tc.addr, err, tc.wantErr)
		}
	}

	// NetworkHintURL and LocalAccessURL
	if got := LocalAccessURL("0.0.0.0:9002", "http"); got != "http://127.0.0.1:9002" {
		t.Errorf("LocalAccessURL: %s", got)
	}
	if got := LocalAccessURL("[::]:9443", "https"); got != "https://127.0.0.1:9443" {
		t.Errorf("LocalAccessURL: %s", got)
	}
	if got := LocalAccessURL("localhost:9002", "http"); got != "http://localhost:9002" {
		t.Errorf("LocalAccessURL: %s", got)
	}
	_ = NetworkHintURL("0.0.0.0:9443", "https")
	_ = NetworkHintURL("127.0.0.1:9002", "http")

	// Plural helper
	if plural(1, "host", "hosts") != "host" || plural(2, "host", "hosts") != "hosts" {
		t.Error("plural helper failed")
	}

	// BuildTLSConfig with bad files
	if _, err := BuildTLSConfig("nonexistent-cert.pem", "nonexistent-key.pem"); err == nil {
		t.Error("expected error for nonexistent cert")
	}

	// validExistingCert edge cases
	dir := t.TempDir()
	badCert := filepath.Join(dir, "bad.crt")
	badKey := filepath.Join(dir, "bad.key")
	_ = os.WriteFile(badCert, []byte("bad pem content"), 0o600)
	_ = os.WriteFile(badKey, []byte("bad pem content"), 0o600)
	if validExistingCert(badCert, badKey, "127.0.0.1", time.Now()) {
		t.Error("validExistingCert should fail for bad PEM")
	}

	goodCert, goodKey, err := EnsureSelfSignedCert(dir, "127.0.0.1:9443")
	if err != nil {
		t.Fatal(err)
	}
	if !validExistingCert(goodCert, goodKey, "127.0.0.1:9443", time.Now()) {
		t.Error("validExistingCert should succeed for valid cert")
	}
	if validExistingCert(goodCert, goodKey, "127.0.0.1:9443", time.Now().Add(400*24*time.Hour)) {
		t.Error("validExistingCert should fail for expired cert")
	}
	if validExistingCert(goodCert, goodKey, "10.254.254.254:9443", time.Now()) {
		t.Error("validExistingCert should fail for mismatched SAN IP")
	}
}
