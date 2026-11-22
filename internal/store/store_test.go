// Tests the file-backed AuthStore: persistence, setup races, role guards, reset codes and sessions.

package store

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSetupPersistenceAndRace(t *testing.T) {
	dir := t.TempDir()
	st, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if st.Dir() != dir {
		t.Fatalf("Dir() = %q, want %q", st.Dir(), dir)
	}
	if n, err := st.CountUsers(); err != nil || n != 0 {
		t.Fatalf("empty count = %d, %v", n, err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	if err := st.ReplaceSetupCode("hash-1", now); err != nil {
		t.Fatal(err)
	}
	if err := st.ReplaceSetupCode("hash-2", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CompleteSetup("hash-1", User{Username: "alice", PasswordHash: "h"}, now); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("old setup code should fail, got %v", err)
	}

	// Only one concurrent CompleteSetup caller may succeed.
	var wins atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := st.CompleteSetup("hash-2", User{Username: "Alice", PasswordHash: "pw"}, now); err == nil {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("expected exactly 1 setup win, got %d", wins.Load())
	}

	// Reopen from disk to verify persistence across restarts.
	reopened, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.ReplaceSetupCode("hash-3", now); !errors.Is(err, ErrSetupClosed) {
		t.Fatalf("setup should be closed, got %v", err)
	}
	if _, err := reopened.CompleteSetup("hash-2", User{Username: "bob", PasswordHash: "pw"}, now); !errors.Is(err, ErrSetupClosed) {
		t.Fatalf("second setup should fail, got %v", err)
	}
	u, err := reopened.GetUserByUsername("ALICE")
	if err != nil {
		t.Fatal(err)
	}
	if u.Username != "alice" || u.Role != RoleAdmin || u.PasswordHash != "pw" {
		t.Fatalf("unexpected user: %+v", u)
	}
	if _, err := reopened.GetUserByID(u.ID); err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
}

func TestUsersRolesAndLastAdminGuard(t *testing.T) {
	st, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	admin, err := st.CreateUser(User{Username: "admin", Role: RoleAdmin, PasswordHash: "h1", CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser(User{Username: "ADMIN", Role: RoleViewer, PasswordHash: "h2"}); !errors.Is(err, ErrExists) {
		t.Fatalf("duplicate username should return ErrExists, got %v", err)
	}
	if _, err := st.UpdateUserRole(admin.ID, RoleViewer, now); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("demoting last admin: %v", err)
	}
	if err := st.DeleteUser(admin.ID); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("deleting last admin: %v", err)
	}

	bob, err := st.CreateUserWithResetCode(User{Username: "bob", Role: RoleScanner, CreatedAt: now.Add(time.Second)}, "invite-hash", now, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := st.HasActiveResetCode(bob.ID, now); err != nil || !ok {
		t.Fatalf("expected active reset code, got %v %v", ok, err)
	}
	if _, err := st.UpdateUserRole(bob.ID, RoleAdmin, now); err != nil {
		t.Fatal(err)
	}
	// Now admin can be demoted because bob is also admin.
	if u, err := st.UpdateUserRole(admin.ID, RoleViewer, now); err != nil || u.Role != RoleViewer {
		t.Fatalf("demote with second admin: %+v %v", u, err)
	}
	if err := st.UpdatePassword(admin.ID, "new-hash", now); err != nil {
		t.Fatal(err)
	}
	users, err := st.ListUsers()
	if err != nil || len(users) != 2 || users[0].Username != "admin" || users[1].Username != "bob" {
		t.Fatalf("ListUsers = %+v, %v", users, err)
	}

	if err := st.CreateSession(Session{TokenHash: "tok-a", UserID: admin.ID, CreatedAt: now, ExpiresAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteUser(admin.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.GetSession("tok-a", now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted user's session should be gone, got %v", err)
	}
}

func TestResetCodesAndCrossProcessVisibility(t *testing.T) {
	dir := t.TempDir()
	serverStore, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	cliStore, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	u, err := serverStore.CreateUser(User{Username: "alice", Role: RoleAdmin, PasswordHash: "old"})
	if err != nil {
		t.Fatal(err)
	}
	if err := serverStore.CreateSession(Session{TokenHash: "sess-1", UserID: u.ID, CreatedAt: now, ExpiresAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}

	// Separate CLI store instance saves reset code while serverStore is alive.
	if err := cliStore.SaveResetCode(u.ID, "reset-hash", now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	// Server immediately observes revoked session and new reset code.
	if _, _, err := serverStore.GetSession("sess-1", now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("session should be revoked after SaveResetCode, got %v", err)
	}
	if _, err := serverStore.ConsumeResetCode("alice", "wrong", "new", now); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("wrong code: %v", err)
	}
	if _, err := serverStore.ConsumeResetCode("nobody", "reset-hash", "new", now); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("unknown user: %v", err)
	}

	updated, err := serverStore.ConsumeResetCode("alice", "reset-hash", "new-hash", now.Add(5*time.Minute))
	if err != nil || updated.PasswordHash != "new-hash" {
		t.Fatalf("ConsumeResetCode: %+v %v", updated, err)
	}
	// Code cannot be reused.
	if _, err := serverStore.ConsumeResetCode("alice", "reset-hash", "newer", now.Add(6*time.Minute)); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("reused code should fail, got %v", err)
	}

	// Expired code fails with ErrCodeExpired and is cleaned up.
	if err := cliStore.SaveResetCode(u.ID, "exp-hash", now, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := serverStore.ConsumeResetCode("alice", "exp-hash", "pw", now.Add(2*time.Minute)); !errors.Is(err, ErrCodeExpired) {
		t.Fatalf("expired code: %v", err)
	}
}

func TestSessionsAndErrors(t *testing.T) {
	dir := t.TempDir()
	st, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	u, err := st.CreateUser(User{Username: "alice", Role: RoleAdmin, PasswordHash: "h"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateSession(Session{TokenHash: "expired", UserID: u.ID, CreatedAt: now.Add(-2 * time.Hour), ExpiresAt: now.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateSession(Session{TokenHash: "valid", UserID: u.ID, CreatedAt: now, ExpiresAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.GetSession("expired", now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired session: %v", err)
	}
	sess, gotU, err := st.GetSession("valid", now)
	if err != nil || sess.UserID != u.ID || gotU.Username != "alice" {
		t.Fatalf("valid session: %+v %+v %v", sess, gotU, err)
	}
	if err := st.DeleteSession("valid"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.GetSession("valid", now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted session: %v", err)
	}

	if err := st.CreateSession(Session{TokenHash: "s1", UserID: u.ID, CreatedAt: now, ExpiresAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteUserSessions(u.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.GetSession("s1", now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteUserSessions: %v", err)
	}

	// Corrupt JSON returns an error instead of silently resetting state.
	if err := os.WriteFile(filepath.Join(dir, authFileName), []byte("{corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CountUsers(); err == nil {
		t.Fatal("expected error for corrupt auth.json")
	}

	// Role helpers and DefaultDataDir.
	if !RoleAdmin.Valid() || !RoleScanner.CanScan() || RoleViewer.CanScan() || Role("bogus").Valid() {
		t.Error("role helpers wrong")
	}
	t.Setenv("NUQS_DATA_DIR", filepath.Join(dir, "custom"))
	if got := DefaultDataDir(); got != filepath.Join(dir, "custom") {
		t.Errorf("DefaultDataDir() = %q", got)
	}
}

func TestStoreCoverageAndEdgeCases(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NUQS_DATA_DIR", dir)
	if got := DefaultDataDir(); got != dir {
		t.Fatalf("DefaultDataDir: %q", got)
	}
	st, err := NewFileStore("")
	if err != nil || st.Dir() != dir {
		t.Fatalf("NewFileStore(\"\") failed: %v", err)
	}

	// Missing user queries.
	if _, err := st.GetUserByID("missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetUserByID missing: %v", err)
	}
	if _, err := st.GetUserByUsername("missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetUserByUsername missing: %v", err)
	}
	if _, err := st.UpdateUserRole("missing", RoleScanner, time.Now()); !errors.Is(err, ErrNotFound) {
		t.Errorf("UpdateUserRole missing: %v", err)
	}
	if err := st.UpdatePassword("missing", "hash", time.Now()); !errors.Is(err, ErrNotFound) {
		t.Errorf("UpdatePassword missing: %v", err)
	}
	if err := st.DeleteUser("missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("DeleteUser missing: %v", err)
	}
	if has, err := st.HasActiveResetCode("missing", time.Now()); err != nil || has {
		t.Errorf("HasActiveResetCode missing: %v %v", has, err)
	}
	if err := st.SaveResetCode("missing", "hash", time.Now(), time.Now().Add(time.Hour)); !errors.Is(err, ErrNotFound) {
		t.Errorf("SaveResetCode missing: %v", err)
	}

	// Complete initial setup.
	now := time.Now().UTC()
	_ = st.ReplaceSetupCode("code1", now)
	admin, err := st.CompleteSetup("code1", User{Username: "Alice", PasswordHash: "pw"}, now)
	if err != nil {
		t.Fatal(err)
	}
	// Setup is locked once a user exists.
	if _, err := st.CompleteSetup("code1", User{Username: "other"}, now); !errors.Is(err, ErrSetupClosed) {
		t.Errorf("CompleteSetup after setup: %v", err)
	}

	// Duplicate username creation.
	if _, err := st.CreateUserWithResetCode(User{Username: "alice", Role: RoleScanner}, "hash", now, now.Add(time.Hour)); !errors.Is(err, ErrExists) {
		t.Errorf("CreateUserWithResetCode duplicate: %v", err)
	}

	// Reset code edge cases.
	if _, err := st.ConsumeResetCode("alice", "wrong-hash", "new-pw", now); !errors.Is(err, ErrInvalidCode) {
		t.Errorf("ConsumeResetCode wrong code: %v", err)
	}
	if _, err := st.ConsumeResetCode("missing", "code", "new-pw", now); !errors.Is(err, ErrInvalidCode) {
		t.Errorf("ConsumeResetCode missing user: %v", err)
	}
	_ = st.SaveResetCode(admin.ID, "expired-code", now.Add(-2*time.Hour), now.Add(-time.Hour))
	if _, err := st.ConsumeResetCode("alice", "expired-code", "new-pw", now); !errors.Is(err, ErrCodeExpired) {
		t.Errorf("ConsumeResetCode expired: %v", err)
	}

	// DefaultDataDir when NUQS_DATA_DIR is unset.
	t.Setenv("NUQS_DATA_DIR", "")
	if base := DefaultDataDir(); base == "" {
		t.Error("DefaultDataDir empty")
	}
}
