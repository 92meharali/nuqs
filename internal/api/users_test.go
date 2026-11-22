// Tests API authentication, RBAC across roles, scan attribution and /api/users management.

package api

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nuqs-org/nuqs/internal/auth"
	"github.com/nuqs-org/nuqs/internal/store"
)

func newAuthedScansHandler(t *testing.T, https bool) (*Handler, string, string, string) {
	t.Helper()
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	authSvc := auth.NewService(st, auth.Options{HTTPS: https, Iterations: 1000})
	code, _, err := authSvc.EnsureSetupCode()
	if err != nil {
		t.Fatal(err)
	}
	_, adminTok, _, err := authSvc.CompleteSetup("127.0.0.1", code, "alice", "alice-password-1")
	if err != nil {
		t.Fatal(err)
	}
	_, scInvite, err := authSvc.CreateUser("bob", store.RoleScanner)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := authSvc.CompletePasswordReset("127.0.0.1", "bob", scInvite.Code, "bob-password-123"); err != nil {
		t.Fatal(err)
	}
	_, scannerTok, _, err := authSvc.Login("127.0.0.1", "bob", "bob-password-123")
	if err != nil {
		t.Fatal(err)
	}
	_, vwInvite, err := authSvc.CreateUser("carol", store.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := authSvc.CompletePasswordReset("127.0.0.1", "carol", vwInvite.Code, "carol-password-1"); err != nil {
		t.Fatal(err)
	}
	_, viewerTok, _, err := authSvc.Login("127.0.0.1", "carol", "carol-password-1")
	if err != nil {
		t.Fatal(err)
	}

	h := newScansHandler(t, &fakeRunner{xml: []byte(telnetHostXML)})
	h.Auth = authSvc
	h.HTTPS = https
	return h, adminTok, scannerTok, viewerTok
}

func doAuthed(h http.Handler, method, url, origin, token, body string, useTLS bool) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, url, strings.NewReader(body))
	if useTLS {
		r.TLS = &tls.ConnectionState{}
	}
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		r.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestAPIRBACAndStartedBy(t *testing.T) {
	h, adminTok, scannerTok, viewerTok := newAuthedScansHandler(t, true)
	const base = "https://192.168.1.50:9002"
	scanBody := `{"name":"LAN check","kind":"full","settings":{"target":"127.0.0.1"}}`

	// Unauthenticated requests are rejected with 401.
	if w := doAuthed(h, http.MethodGet, base+"/api/status", "", "", "", true); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauth status = %d, want 401", w.Code)
	}
	if w := doAuthed(h, http.MethodPost, base+"/api/scans", base, "", scanBody, true); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauth start = %d, want 401", w.Code)
	}

	// Viewer can read status/scans, but cannot start or stop scans, nor manage users.
	if w := doAuthed(h, http.MethodGet, base+"/api/status", "", viewerTok, "", true); w.Code != http.StatusOK {
		t.Fatalf("viewer status = %d", w.Code)
	}
	if w := doAuthed(h, http.MethodPost, base+"/api/scans", base, viewerTok, scanBody, true); w.Code != http.StatusForbidden {
		t.Fatalf("viewer start = %d, want 403", w.Code)
	}
	if w := doAuthed(h, http.MethodGet, base+"/api/users", "", viewerTok, "", true); w.Code != http.StatusForbidden {
		t.Fatalf("viewer list users = %d, want 403", w.Code)
	}

	// Scanner can start scans over HTTPS from LAN origin, and scan records started_by="bob".
	w := doAuthed(h, http.MethodPost, base+"/api/scans", base, scannerTok, scanBody, true)
	if w.Code != http.StatusCreated {
		t.Fatalf("scanner start = %d %s", w.Code, w.Body.String())
	}
	var created ScanSummary
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.StartedBy != "bob" {
		t.Errorf("StartedBy = %q, want bob", created.StartedBy)
	}
	waitDone(t, h, created.ID)

	// Spoofing started_by in JSON is rejected by DisallowUnknownFields.
	if w := doAuthed(h, http.MethodPost, base+"/api/scans", base, scannerTok, `{"started_by":"alice","kind":"full","settings":{"target":"127.0.0.1"}}`, true); w.Code != http.StatusBadRequest {
		t.Errorf("spoofed started_by = %d, want 400", w.Code)
	}
	// Viewer cannot stop a scan.
	if w := doAuthed(h, http.MethodPost, base+"/api/scans/1/stop", base, viewerTok, "", true); w.Code != http.StatusForbidden {
		t.Errorf("viewer stop = %d, want 403", w.Code)
	}
	// Scanner cannot manage users.
	if w := doAuthed(h, http.MethodGet, base+"/api/users", "", scannerTok, "", true); w.Code != http.StatusForbidden {
		t.Errorf("scanner users = %d, want 403", w.Code)
	}

	// Admin manages users via /api/users.
	w = doAuthed(h, http.MethodGet, base+"/api/users", "", adminTok, "", true)
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "pbkdf2") {
		t.Fatalf("admin list users: %d %s", w.Code, w.Body.String())
	}
	w = doAuthed(h, http.MethodPost, base+"/api/users", base, adminTok, `{"username":"dave","role":"viewer"}`, true)
	if w.Code != http.StatusCreated {
		t.Fatalf("create user = %d %s", w.Code, w.Body.String())
	}
	var added createUserResponse
	if err := json.Unmarshal(w.Body.Bytes(), &added); err != nil || added.ResetCode == "" || added.User.Username != "dave" {
		t.Fatalf("unexpected create response: %+v %v", added, err)
	}
	// Update role.
	w = doAuthed(h, http.MethodPatch, base+"/api/users/"+added.User.ID, base, adminTok, `{"role":"scanner"}`, true)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"scanner"`) {
		t.Fatalf("patch role = %d %s", w.Code, w.Body.String())
	}
	// Reset password.
	w = doAuthed(h, http.MethodPost, base+"/api/users/"+added.User.ID+"/reset-password", base, adminTok, "", true)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"reset_code"`) {
		t.Fatalf("reset password = %d %s", w.Code, w.Body.String())
	}
	// Cannot delete self.
	alice, _ := h.Auth.Store.GetUserByUsername("alice")
	if w := doAuthed(h, http.MethodDelete, base+"/api/users/"+alice.ID, base, adminTok, "", true); w.Code != http.StatusBadRequest {
		t.Errorf("self delete = %d, want 400", w.Code)
	}
	// Cannot demote last admin.
	if w := doAuthed(h, http.MethodPatch, base+"/api/users/"+alice.ID, base, adminTok, `{"role":"viewer"}`, true); w.Code != http.StatusConflict {
		t.Errorf("demote last admin = %d, want 409", w.Code)
	}
	// Foreign origin rejected on user mutations.
	if w := doAuthed(h, http.MethodDelete, base+"/api/users/"+added.User.ID, "https://evil.example", adminTok, "", true); w.Code != http.StatusForbidden {
		t.Errorf("foreign origin delete = %d, want 403", w.Code)
	}
	// Delete teammate succeeds.
	if w := doAuthed(h, http.MethodDelete, base+"/api/users/"+added.User.ID, base, adminTok, "", true); w.Code != http.StatusNoContent {
		t.Errorf("delete teammate = %d, want 204", w.Code)
	}
	// Error status branches on /api/users.
	if w := doAuthed(h, http.MethodPost, base+"/api/users", base, adminTok, `{"username":"x","role":"viewer"}`, true); w.Code != http.StatusBadRequest {
		t.Errorf("short username = %d, want 400", w.Code)
	}
	if w := doAuthed(h, http.MethodPost, base+"/api/users", base, adminTok, `not-json`, true); w.Code != http.StatusBadRequest {
		t.Errorf("bad json = %d, want 400", w.Code)
	}
	if w := doAuthed(h, http.MethodPatch, base+"/api/users/missing", base, adminTok, `{"role":"viewer"}`, true); w.Code != http.StatusNotFound {
		t.Errorf("patch missing = %d, want 404", w.Code)
	}
	if w := doAuthed(h, http.MethodPatch, base+"/api/users/"+alice.ID, base, adminTok, `{"role":"bogus"}`, true); w.Code != http.StatusBadRequest {
		t.Errorf("patch bad role = %d, want 400", w.Code)
	}
	if w := doAuthed(h, http.MethodPost, base+"/api/users/missing/reset-password", base, adminTok, "", true); w.Code != http.StatusNotFound {
		t.Errorf("reset missing = %d, want 404", w.Code)
	}
	if w := doAuthed(h, http.MethodDelete, base+"/api/users/missing", base, adminTok, "", true); w.Code != http.StatusNotFound {
		t.Errorf("delete missing = %d, want 404", w.Code)
	}
}
