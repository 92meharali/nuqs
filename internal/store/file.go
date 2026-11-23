// File-backed AuthStore persisting accounts, sessions and one-time codes as atomic JSON.

package store

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	authFileName = "auth.json"
	lockFileName = "auth.lock"
	staleLockAge = 30 * time.Second
	lockTimeout  = 5 * time.Second
)

type fileState struct {
	Version  int           `json:"version"`
	Users    []User        `json:"users"`
	Sessions []Session     `json:"sessions"`
	Codes    []OneTimeCode `json:"codes"`
}

// FileStore stores auth state in a private JSON file inside a data directory.
// Every operation acquires an in-process mutex and a cross-process lockfile so
// CLI commands (like "nuqs user reset-password") can safely update state while
// "nuqs serve" is running.
type FileStore struct {
	dir      string
	path     string
	lockPath string
	mu       sync.Mutex
}

// NewFileStore prepares dir (with 0700 permissions) and returns a FileStore.
func NewFileStore(dir string) (*FileStore, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		dir = DefaultDataDir()
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	return &FileStore{
		dir:      dir,
		path:     filepath.Join(dir, authFileName),
		lockPath: filepath.Join(dir, lockFileName),
	}, nil
}

// Dir returns the directory where this store keeps its files.
func (f *FileStore) Dir() string {
	return f.dir
}

// CountUsers returns the number of registered accounts.
func (f *FileStore) CountUsers() (int, error) {
	var n int
	err := f.withState(false, func(st *fileState) error {
		n = len(st.Users)
		return nil
	})
	return n, err
}

// ListUsers returns all accounts sorted by creation time, then username.
func (f *FileStore) ListUsers() ([]User, error) {
	users, _, err := f.ListUsersWithResetStatus(time.Time{})
	return users, err
}

// ListUsersWithResetStatus reads accounts and their active reset flags in one snapshot.
func (f *FileStore) ListUsersWithResetStatus(now time.Time) ([]User, map[string]bool, error) {
	var out []User
	active := map[string]bool{}
	err := f.withState(false, func(st *fileState) error {
		out = slices.Clone(st.Users)
		if !now.IsZero() {
			for _, c := range st.Codes {
				if c.Purpose == CodeReset && (c.ExpiresAt.IsZero() || !c.ExpiresAt.Before(now)) {
					active[c.UserID] = true
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	slices.SortFunc(out, func(a, b User) int {
		if a.CreatedAt.Before(b.CreatedAt) {
			return -1
		}
		if a.CreatedAt.After(b.CreatedAt) {
			return 1
		}
		return strings.Compare(a.Username, b.Username)
	})
	return out, active, nil
}

// GetUserByID returns the user with the given ID or ErrNotFound.
func (f *FileStore) GetUserByID(id string) (User, error) {
	var out User
	err := f.withState(false, func(st *fileState) error {
		i := slices.IndexFunc(st.Users, func(u User) bool { return u.ID == id })
		if i < 0 {
			return ErrNotFound
		}
		out = st.Users[i]
		return nil
	})
	return out, err
}

// GetUserByUsername returns the user matching username (case-insensitive) or ErrNotFound.
func (f *FileStore) GetUserByUsername(username string) (User, error) {
	canon := CanonicalUsername(username)
	var out User
	err := f.withState(false, func(st *fileState) error {
		i := slices.IndexFunc(st.Users, func(u User) bool { return CanonicalUsername(u.Username) == canon })
		if i < 0 {
			return ErrNotFound
		}
		out = st.Users[i]
		return nil
	})
	return out, err
}

// CreateUser inserts a new account and returns it with its assigned ID.
func (f *FileStore) CreateUser(user User) (User, error) {
	var created User
	err := f.withState(true, func(st *fileState) error {
		var err error
		created, err = appendUser(st, user)
		return err
	})
	return created, err
}

// CreateUserWithResetCode atomically inserts a new account and stores an initial
// one-time password setup/reset code for that user.
func (f *FileStore) CreateUserWithResetCode(user User, codeHash string, now, expiresAt time.Time) (User, error) {
	var created User
	err := f.withState(true, func(st *fileState) error {
		u, err := appendUser(st, user)
		if err != nil {
			return err
		}
		st.Codes = append(st.Codes, OneTimeCode{
			CodeHash:  codeHash,
			Purpose:   CodeReset,
			UserID:    u.ID,
			CreatedAt: now.UTC(),
			ExpiresAt: expiresAt.UTC(),
		})
		created = u
		return nil
	})
	return created, err
}

// UpdateUserRole changes a user's role while ensuring at least one admin remains.
func (f *FileStore) UpdateUserRole(id string, role Role, now time.Time) (User, error) {
	if !role.Valid() {
		return User{}, fmt.Errorf("invalid role %q", role)
	}
	var updated User
	err := f.withState(true, func(st *fileState) error {
		i := slices.IndexFunc(st.Users, func(u User) bool { return u.ID == id })
		if i < 0 {
			return ErrNotFound
		}
		if st.Users[i].Role == RoleAdmin && st.Users[i].PasswordHash != "" && role != RoleAdmin && countAdmins(st.Users) <= 1 {
			return ErrLastAdmin
		}
		st.Users[i].Role = role
		st.Users[i].UpdatedAt = now.UTC()
		updated = st.Users[i]
		return nil
	})
	return updated, err
}

// UpdatePassword replaces the stored password hash for user id.
func (f *FileStore) UpdatePassword(id, passwordHash string, now time.Time) error {
	return f.withState(true, func(st *fileState) error {
		i := slices.IndexFunc(st.Users, func(u User) bool { return u.ID == id })
		if i < 0 {
			return ErrNotFound
		}
		st.Users[i].PasswordHash = passwordHash
		st.Users[i].UpdatedAt = now.UTC()
		return nil
	})
}

// RotatePassword commits a password change and session rotation in one write.
func (f *FileStore) RotatePassword(expectedHash, newHash string, session Session) error {
	return f.withState(true, func(st *fileState) error {
		i := slices.IndexFunc(st.Users, func(u User) bool { return u.ID == session.UserID })
		if i < 0 {
			return ErrNotFound
		}
		if !constantEqual(st.Users[i].PasswordHash, expectedHash) {
			return ErrPasswordChanged
		}
		st.Users[i].PasswordHash = newHash
		st.Users[i].UpdatedAt = session.CreatedAt
		st.Sessions = slices.DeleteFunc(st.Sessions, func(s Session) bool {
			return s.UserID == session.UserID || !s.ExpiresAt.After(session.CreatedAt)
		})
		st.Sessions = append(st.Sessions, session)
		return nil
	})
}

// DeleteUser removes a user along with all of their sessions and reset codes.
// Deleting the last remaining admin returns ErrLastAdmin.
func (f *FileStore) DeleteUser(id string) error {
	return f.withState(true, func(st *fileState) error {
		i := slices.IndexFunc(st.Users, func(u User) bool { return u.ID == id })
		if i < 0 {
			return ErrNotFound
		}
		if st.Users[i].Role == RoleAdmin && st.Users[i].PasswordHash != "" && countAdmins(st.Users) <= 1 {
			return ErrLastAdmin
		}
		st.Users = slices.Delete(st.Users, i, i+1)
		st.Sessions = slices.DeleteFunc(st.Sessions, func(s Session) bool { return s.UserID == id })
		st.Codes = slices.DeleteFunc(st.Codes, func(c OneTimeCode) bool { return c.UserID == id })
		return nil
	})
}

// ReplaceSetupCode stores a single active initial-setup code hash when no users exist.
func (f *FileStore) ReplaceSetupCode(codeHash string, now time.Time) error {
	return f.withState(true, func(st *fileState) error {
		if len(st.Users) > 0 {
			return ErrSetupClosed
		}
		st.Codes = slices.DeleteFunc(st.Codes, func(c OneTimeCode) bool { return c.Purpose == CodeSetup })
		st.Codes = append(st.Codes, OneTimeCode{
			CodeHash:  codeHash,
			Purpose:   CodeSetup,
			CreatedAt: now.UTC(),
		})
		return nil
	})
}

// CompleteSetup atomically verifies the setup code, creates the first admin
// account, and invalidates all setup codes.
func (f *FileStore) CompleteSetup(codeHash string, admin User, now time.Time) (User, error) {
	var created User
	err := f.withState(true, func(st *fileState) error {
		if len(st.Users) > 0 {
			return ErrSetupClosed
		}
		matched := false
		for _, c := range st.Codes {
			if c.Purpose == CodeSetup && constantEqual(c.CodeHash, codeHash) {
				matched = true
				break
			}
		}
		if !matched {
			return ErrInvalidCode
		}
		admin.Role = RoleAdmin
		if admin.CreatedAt.IsZero() {
			admin.CreatedAt = now.UTC()
		}
		if admin.UpdatedAt.IsZero() {
			admin.UpdatedAt = now.UTC()
		}
		u, err := appendUser(st, admin)
		if err != nil {
			return err
		}
		st.Codes = slices.DeleteFunc(st.Codes, func(c OneTimeCode) bool { return c.Purpose == CodeSetup })
		created = u
		return nil
	})
	return created, err
}

// SaveResetCode stores a single-use password reset code for userID and revokes
// all active sessions and previous reset codes belonging to that user.
func (f *FileStore) SaveResetCode(userID, codeHash string, now, expiresAt time.Time) error {
	return f.withState(true, func(st *fileState) error {
		if !slices.ContainsFunc(st.Users, func(u User) bool { return u.ID == userID }) {
			return ErrNotFound
		}
		st.Codes = slices.DeleteFunc(st.Codes, func(c OneTimeCode) bool {
			return c.Purpose == CodeReset && c.UserID == userID
		})
		st.Sessions = slices.DeleteFunc(st.Sessions, func(s Session) bool { return s.UserID == userID })
		st.Codes = append(st.Codes, OneTimeCode{
			CodeHash:  codeHash,
			Purpose:   CodeReset,
			UserID:    userID,
			CreatedAt: now.UTC(),
			ExpiresAt: expiresAt.UTC(),
		})
		return nil
	})
}

// ConsumeResetCode atomically validates username's one-time reset code, sets
// newPasswordHash, removes the code and revokes existing sessions.
func (f *FileStore) ConsumeResetCode(username, codeHash, newPasswordHash string, now time.Time) (User, error) {
	canon := CanonicalUsername(username)
	var updated User
	err := f.withState(true, func(st *fileState) error {
		ui := slices.IndexFunc(st.Users, func(u User) bool { return CanonicalUsername(u.Username) == canon })
		if ui < 0 {
			return ErrInvalidCode
		}
		uid := st.Users[ui].ID
		var matched *OneTimeCode
		for i := range st.Codes {
			c := &st.Codes[i]
			if c.Purpose == CodeReset && c.UserID == uid && constantEqual(c.CodeHash, codeHash) {
				matched = c
				break
			}
		}
		if matched == nil {
			return ErrInvalidCode
		}
		if !matched.ExpiresAt.IsZero() && matched.ExpiresAt.Before(now) {
			st.Codes = slices.DeleteFunc(st.Codes, func(c OneTimeCode) bool {
				return c.Purpose == CodeReset && c.UserID == uid
			})
			return ErrCodeExpired
		}
		st.Users[ui].PasswordHash = newPasswordHash
		st.Users[ui].UpdatedAt = now.UTC()
		st.Codes = slices.DeleteFunc(st.Codes, func(c OneTimeCode) bool {
			return c.Purpose == CodeReset && c.UserID == uid
		})
		st.Sessions = slices.DeleteFunc(st.Sessions, func(s Session) bool { return s.UserID == uid })
		updated = st.Users[ui]
		return nil
	})
	return updated, err
}

// HasActiveResetCode reports whether userID has an unexpired password reset code.
func (f *FileStore) HasActiveResetCode(userID string, now time.Time) (bool, error) {
	var active bool
	err := f.withState(false, func(st *fileState) error {
		for _, c := range st.Codes {
			if c.Purpose == CodeReset && c.UserID == userID && (c.ExpiresAt.IsZero() || !c.ExpiresAt.Before(now)) {
				active = true
				return nil
			}
		}
		return nil
	})
	return active, err
}

// CreateSession stores a new session and prunes already-expired sessions.
func (f *FileStore) CreateSession(session Session) error {
	return f.withState(true, func(st *fileState) error {
		now := session.CreatedAt
		if now.IsZero() {
			now = time.Now().UTC()
			session.CreatedAt = now
		}
		st.Sessions = slices.DeleteFunc(st.Sessions, func(s Session) bool {
			return !s.ExpiresAt.After(now) || s.TokenHash == session.TokenHash
		})
		st.Sessions = append(st.Sessions, Session{
			TokenHash: session.TokenHash,
			UserID:    session.UserID,
			CreatedAt: session.CreatedAt.UTC(),
			ExpiresAt: session.ExpiresAt.UTC(),
		})
		return nil
	})
}

// GetSession resolves an active (unexpired) session and its user, or ErrNotFound.
func (f *FileStore) GetSession(tokenHash string, now time.Time) (Session, User, error) {
	var sess Session
	var user User
	err := f.withState(false, func(st *fileState) error {
		var found *Session
		for i := range st.Sessions {
			if constantEqual(st.Sessions[i].TokenHash, tokenHash) {
				found = &st.Sessions[i]
				break
			}
		}
		if found == nil || !found.ExpiresAt.After(now) {
			return ErrNotFound
		}
		ui := slices.IndexFunc(st.Users, func(u User) bool { return u.ID == found.UserID })
		if ui < 0 {
			return ErrNotFound
		}
		sess = *found
		user = st.Users[ui]
		return nil
	})
	return sess, user, err
}

// DeleteSession removes a single session by token hash.
func (f *FileStore) DeleteSession(tokenHash string) error {
	return f.withState(true, func(st *fileState) error {
		st.Sessions = slices.DeleteFunc(st.Sessions, func(s Session) bool {
			return constantEqual(s.TokenHash, tokenHash)
		})
		return nil
	})
}

// DeleteUserSessions revokes every active session belonging to userID.
func (f *FileStore) DeleteUserSessions(userID string) error {
	return f.withState(true, func(st *fileState) error {
		st.Sessions = slices.DeleteFunc(st.Sessions, func(s Session) bool {
			return s.UserID == userID
		})
		return nil
	})
}

func appendUser(st *fileState, user User) (User, error) {
	canon := CanonicalUsername(user.Username)
	if canon == "" {
		return User{}, errors.New("username is required")
	}
	if !user.Role.Valid() {
		return User{}, fmt.Errorf("invalid role %q", user.Role)
	}
	if slices.ContainsFunc(st.Users, func(u User) bool { return CanonicalUsername(u.Username) == canon }) {
		return User{}, ErrExists
	}
	if user.ID == "" {
		id, err := randomID()
		if err != nil {
			return User{}, err
		}
		user.ID = id
	}
	now := time.Now().UTC()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	if user.UpdatedAt.IsZero() {
		user.UpdatedAt = user.CreatedAt
	}
	user.Username = canon
	user.CreatedAt = user.CreatedAt.UTC()
	user.UpdatedAt = user.UpdatedAt.UTC()
	st.Users = append(st.Users, user)
	return user, nil
}

func countAdmins(users []User) int {
	n := 0
	for _, u := range users {
		if u.Role == RoleAdmin && u.PasswordHash != "" {
			n++
		}
	}
	return n
}

func constantEqual(a, b string) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func randomID() (string, error) {
	var buf [12]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(buf[:]), nil
}

func (f *FileStore) withState(write bool, fn func(*fileState) error) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	unlock, err := f.acquireFileLock()
	if err != nil {
		return err
	}
	defer unlock()

	st, err := f.readState()
	if err != nil {
		return err
	}
	if err := fn(&st); err != nil {
		// Persist cleanup even when an expired code was pruned during validation.
		if write && errors.Is(err, ErrCodeExpired) {
			_ = f.writeState(st)
		}
		return err
	}
	if write {
		return f.writeState(st)
	}
	return nil
}

func (f *FileStore) readState() (fileState, error) {
	data, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return fileState{Version: 1}, nil
	}
	if err != nil {
		return fileState{}, fmt.Errorf("read auth store: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return fileState{Version: 1}, nil
	}
	var st fileState
	if err := json.Unmarshal(data, &st); err != nil {
		return fileState{}, fmt.Errorf("parse auth store: %w", err)
	}
	if st.Version == 0 {
		st.Version = 1
	}
	return st, nil
}

func (f *FileStore) writeState(st fileState) error {
	st.Version = 1
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("encode auth store: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(f.dir, ".auth-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp auth file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	_ = tmp.Chmod(0o600)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp auth file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp auth file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp auth file: %w", err)
	}
	if err := os.Rename(tmpPath, f.path); err != nil {
		return fmt.Errorf("commit auth store: %w", err)
	}
	_ = os.Chmod(f.path, 0o600)
	cleanup = false
	return nil
}

func (f *FileStore) acquireFileLock() (func(), error) {
	deadline := time.Now().Add(lockTimeout)
	for {
		lockFile, err := os.OpenFile(f.lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_ = lockFile.Close()
			return func() { _ = os.Remove(f.lockPath) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("acquire auth store lock: %w", err)
		}
		if info, statErr := os.Stat(f.lockPath); statErr == nil && time.Since(info.ModTime()) > staleLockAge {
			_ = os.Remove(f.lockPath)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for auth store lock %s", f.lockPath)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
