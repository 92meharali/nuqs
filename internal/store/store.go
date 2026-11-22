// Package store persists Nuqs accounts, sessions and one-time recovery codes.
//
// All auth operations go through the AuthStore interface so the storage engine
// can evolve (for example, from a local file to SQLite) without changing auth,
// API or web UI code.
package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Role controls what a signed-in user may do in the web UI and API.
type Role string

// Supported account roles.
const (
	RoleAdmin   Role = "admin"
	RoleScanner Role = "scanner"
	RoleViewer  Role = "viewer"
)

// Valid reports whether r is one of the recognized roles.
func (r Role) Valid() bool {
	return r == RoleAdmin || r == RoleScanner || r == RoleViewer
}

// CanScan reports whether a user with role r may start or stop scans.
func (r Role) CanScan() bool {
	return r == RoleAdmin || r == RoleScanner
}

// CodePurpose distinguishes initial setup codes from password reset codes.
type CodePurpose string

// Supported one-time code purposes.
const (
	CodeSetup CodePurpose = "setup"
	CodeReset CodePurpose = "reset"
)

// Sentinel errors returned by AuthStore implementations.
var (
	ErrNotFound    = errors.New("not found")
	ErrExists      = errors.New("already exists")
	ErrLastAdmin   = errors.New("cannot remove or demote the last admin")
	ErrInvalidCode = errors.New("invalid one-time code")
	ErrCodeExpired = errors.New("one-time code has expired")
	ErrSetupClosed = errors.New("initial setup is already complete")
)

// User is a persisted Nuqs account.
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Role         Role      `json:"role"`
	PasswordHash string    `json:"password_hash,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Session is an active authenticated browser session stored as a token hash.
type Session struct {
	TokenHash string    `json:"token_hash"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// OneTimeCode is a single-use hashed code for first-run setup or password reset.
type OneTimeCode struct {
	CodeHash  string      `json:"code_hash"`
	Purpose   CodePurpose `json:"purpose"`
	UserID    string      `json:"user_id,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
	ExpiresAt time.Time   `json:"expires_at,omitempty"`
}

// AuthStore is the persistence contract for accounts, sessions and recovery codes.
type AuthStore interface {
	CountUsers() (int, error)
	ListUsers() ([]User, error)
	GetUserByID(id string) (User, error)
	GetUserByUsername(username string) (User, error)
	CreateUser(user User) (User, error)
	CreateUserWithResetCode(user User, codeHash string, now, expiresAt time.Time) (User, error)
	UpdateUserRole(id string, role Role, now time.Time) (User, error)
	UpdatePassword(id, passwordHash string, now time.Time) error
	DeleteUser(id string) error

	ReplaceSetupCode(codeHash string, now time.Time) error
	CompleteSetup(codeHash string, admin User, now time.Time) (User, error)

	SaveResetCode(userID, codeHash string, now, expiresAt time.Time) error
	ConsumeResetCode(username, codeHash, newPasswordHash string, now time.Time) (User, error)
	HasActiveResetCode(userID string, now time.Time) (bool, error)

	CreateSession(session Session) error
	GetSession(tokenHash string, now time.Time) (Session, User, error)
	DeleteSession(tokenHash string) error
	DeleteUserSessions(userID string) error
}

// CanonicalUsername normalizes a username for case-insensitive matching.
func CanonicalUsername(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// DefaultDataDir returns the default directory for Nuqs state files.
// NUQS_DATA_DIR overrides the platform user config directory.
func DefaultDataDir() string {
	if env := strings.TrimSpace(os.Getenv("NUQS_DATA_DIR")); env != "" {
		return env
	}
	if base, err := os.UserConfigDir(); err == nil && base != "" {
		return filepath.Join(base, "nuqs")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".nuqs")
	}
	return ".nuqs"
}
