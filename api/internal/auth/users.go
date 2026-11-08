package auth

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

var (
	// ErrSetupDone means an admin already exists, so first-run setup is refused.
	ErrSetupDone = errors.New("setup already done: an admin exists")
	// ErrInvalidCredentials means the username is unknown or the password is wrong; callers
	// must not tell the two apart.
	ErrInvalidCredentials = errors.New("wrong username or password")
	// ErrNotFound means no row has the given ID or username.
	ErrNotFound = errors.New("not found")
	// ErrUsernameTaken means another admin has the username, compared ignoring case.
	ErrUsernameTaken = errors.New("username taken")
	// ErrLastAdmin means the admin is the only one left, so it cannot be removed (FR-011).
	ErrLastAdmin = errors.New("the last admin cannot be removed")
	// ErrWrongPassword means the current password given for a change is wrong.
	ErrWrongPassword = errors.New("current password is wrong")
)

// Admin is an admin account as shown to clients. It never holds password data.
type Admin struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

// AdminCount returns how many admins exist.
func AdminCount(db *sql.DB) (int, error) {
	var n int
	err := db.QueryRow("SELECT count(*) FROM users").Scan(&n)
	return n, err
}

// CreateFirstAdmin creates the first admin with username (trimmed) and password at now, only
// while no admin exists. It returns ErrSetupDone when an admin exists, a ValidationError for
// input that breaks the rules, and ErrBusy when no hash slot frees up. Concurrent calls create
// at most one admin (FR-004).
func CreateFirstAdmin(db *sql.DB, username, password string, now time.Time) (Admin, error) {
	// Checked before hashing so a public caller cannot make a finished install hash.
	if n, err := AdminCount(db); err != nil || n > 0 {
		if err == nil {
			err = ErrSetupDone
		}
		return Admin{}, err
	}
	username, err := ValidateUsername(username)
	if err != nil {
		return Admin{}, err
	}
	if err := ValidatePassword(password); err != nil {
		return Admin{}, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return Admin{}, err
	}
	// One statement, so the emptiness check and the insert cannot interleave with another setup.
	res, err := db.Exec("INSERT INTO users (username, password_hash, created_at) SELECT ?, ?, ? WHERE NOT EXISTS (SELECT 1 FROM users)",
		username, hash, now.Unix())
	if err != nil {
		return Admin{}, err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		if err == nil {
			err = ErrSetupDone
		}
		return Admin{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Admin{}, err
	}
	return Admin{ID: id, Username: username, CreatedAt: unixTime(now.Unix())}, nil
}

// Authenticate returns the admin whose username (trimmed, compared ignoring case) and password
// match. Unknown usernames and wrong passwords both cost one password verify and both return
// ErrInvalidCredentials (FR-014). It returns a ValidationError for input that breaks the
// username or password rules, before any hashing, and ErrBusy when no hash slot frees up.
func Authenticate(db *sql.DB, username, password string) (Admin, error) {
	username, err := ValidateUsername(username)
	if err != nil {
		return Admin{}, err
	}
	if err := ValidatePassword(password); err != nil {
		return Admin{}, err
	}
	var a Admin
	var created int64
	var hash string
	err = db.QueryRow("SELECT id, username, password_hash, created_at FROM users WHERE username = ?", username).
		Scan(&a.ID, &a.Username, &hash, &created)
	if errors.Is(err, sql.ErrNoRows) {
		if err := DummyVerify(password); err != nil {
			return Admin{}, err
		}
		return Admin{}, ErrInvalidCredentials
	}
	if err != nil {
		return Admin{}, err
	}
	ok, err := VerifyPassword(hash, password)
	if err != nil {
		return Admin{}, err
	}
	if !ok {
		return Admin{}, ErrInvalidCredentials
	}
	a.CreatedAt = unixTime(created)
	return a, nil
}

// GetAdmin returns the admin with id, or ErrNotFound.
func GetAdmin(db *sql.DB, id int64) (Admin, error) {
	var a Admin
	var created int64
	err := db.QueryRow("SELECT id, username, created_at FROM users WHERE id = ?", id).Scan(&a.ID, &a.Username, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return Admin{}, ErrNotFound
	}
	if err != nil {
		return Admin{}, err
	}
	a.CreatedAt = unixTime(created)
	return a, nil
}

// AddAdmin creates an admin with username (trimmed) and password at now. It returns
// ErrUsernameTaken when the username exists ignoring case, a ValidationError for input that
// breaks the rules, and ErrBusy when no hash slot frees up.
func AddAdmin(db *sql.DB, username, password string, now time.Time) (Admin, error) {
	username, err := ValidateUsername(username)
	if err != nil {
		return Admin{}, err
	}
	if err := ValidatePassword(password); err != nil {
		return Admin{}, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return Admin{}, err
	}
	// The NOCASE column makes "=" ignore case; one statement keeps concurrent adds from racing.
	res, err := db.Exec("INSERT INTO users (username, password_hash, created_at) SELECT ?, ?, ? WHERE NOT EXISTS (SELECT 1 FROM users WHERE username = ?)",
		username, hash, now.Unix(), username)
	if err != nil {
		return Admin{}, err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		if err == nil {
			err = ErrUsernameTaken
		}
		return Admin{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Admin{}, err
	}
	return Admin{ID: id, Username: username, CreatedAt: unixTime(now.Unix())}, nil
}

// ListAdmins returns every admin, oldest first, with no password data.
func ListAdmins(db *sql.DB) ([]Admin, error) {
	rows, err := db.Query("SELECT id, username, created_at FROM users ORDER BY created_at, id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	admins := []Admin{}
	for rows.Next() {
		var a Admin
		var created int64
		if err := rows.Scan(&a.ID, &a.Username, &created); err != nil {
			return nil, err
		}
		a.CreatedAt = unixTime(created)
		admins = append(admins, a)
	}
	return admins, rows.Err()
}

// RemoveAdmin deletes admin id, and by cascade all its sessions, and returns its username.
// It returns ErrLastAdmin when id is the only admin and ErrNotFound for an unknown id.
// Concurrent removals always leave at least one admin.
func RemoveAdmin(db *sql.DB, id int64) (string, error) {
	var username string
	// The count is checked in the same statement, so two removals cannot both pass it.
	err := db.QueryRow("DELETE FROM users WHERE id = ? AND (SELECT COUNT(*) FROM users) > 1 RETURNING username", id).Scan(&username)
	if !errors.Is(err, sql.ErrNoRows) {
		return username, err
	}
	if _, err := GetAdmin(db, id); err != nil {
		return "", err
	}
	return "", ErrLastAdmin
}

// ChangePassword sets admin userID's password to next after checking current, then ends the
// admin's other sessions, keeping keepSessionID, in the same transaction (FR-012). It returns
// a ValidationError for a bad next password (before any hashing), ErrWrongPassword when
// current is wrong (nothing changes), ErrNotFound for an unknown admin and ErrBusy when no
// hash slot frees up.
func ChangePassword(db *sql.DB, userID int64, current, next, keepSessionID string) error {
	if err := ValidatePassword(next); err != nil {
		return err
	}
	var hash string
	err := db.QueryRow("SELECT password_hash FROM users WHERE id = ?", userID).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	ok, err := VerifyPassword(hash, current)
	if err != nil {
		return err
	}
	if !ok {
		return ErrWrongPassword
	}
	if hash, err = HashPassword(next); err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("UPDATE users SET password_hash = ? WHERE id = ?", hash, userID); err != nil {
		return err
	}
	if err := DeleteOtherSessions(tx, userID, keepSessionID); err != nil {
		return err
	}
	return tx.Commit()
}

// ResetPassword sets the password of the admin named username (compared ignoring case) and
// ends all of that admin's sessions, in one transaction. It is the operator's recovery path
// (reset-password command) and needs no current password, so it must never be reachable over
// HTTP (FR-042). It returns a ValidationError for a bad password, ErrNotFound for an unknown
// username (nothing changes) and ErrBusy when no hash slot frees up.
func ResetPassword(db *sql.DB, username, password string) error {
	if err := ValidatePassword(password); err != nil {
		return err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRow("UPDATE users SET password_hash = ? WHERE username = ? RETURNING id", hash, strings.TrimSpace(username)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if err := DeleteUserSessions(tx, id); err != nil {
		return err
	}
	return tx.Commit()
}
