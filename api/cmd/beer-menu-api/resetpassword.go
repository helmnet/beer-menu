package main

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/helmnet/beer-menu/api/internal/auth"
	"github.com/helmnet/beer-menu/api/internal/store"
)

// resetPassword runs "beer-menu-api reset-password <username>" (research R10), the operator's
// recovery when every admin is locked out. args are the words after the subcommand. It refuses
// unless fd is a terminal, reads the new password twice from fd with echo off, checks the
// entries match and follow the password rules, opens the existing database in dataDir and refuses
// unless the API has migrated it to the newest schema, then sets the password and ends all of
// that admin's sessions. Prompts and errors go to out; neither they nor the log ever hold the
// password. It returns the exit code: 0 on success, 1 with nothing changed otherwise.
func resetPassword(dataDir string, args []string, fd int, out io.Writer, isTerminal func(int) bool, readPassword func(int) ([]byte, error)) int {
	if err := runResetPassword(dataDir, args, fd, out, isTerminal, readPassword); err != nil {
		fmt.Fprintln(out, "reset-password:", err)
		return 1
	}
	fmt.Fprintln(out, "Password reset. All of that admin's sessions have ended.")
	return 0
}

func runResetPassword(dataDir string, args []string, fd int, out io.Writer, isTerminal func(int) bool, readPassword func(int) ([]byte, error)) error {
	if len(args) != 1 {
		return errors.New("usage: beer-menu-api reset-password <username>")
	}
	username := args[0]
	// Without a terminal the password would echo, or come from a pipe or shell history.
	if !isTerminal(fd) {
		return errors.New("stdin is not a terminal; run it with docker compose exec -it")
	}
	fmt.Fprint(out, "New password: ")
	first, err := readPassword(fd)
	fmt.Fprintln(out)
	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}
	fmt.Fprint(out, "Repeat new password: ")
	second, err := readPassword(fd)
	fmt.Fprintln(out)
	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}
	password := string(first)
	if password != string(second) {
		return errors.New("the passwords do not match; nothing changed")
	}
	if err := auth.ValidatePassword(password); err != nil {
		return err
	}
	// store.Open would create an empty database, so a wrong data folder is refused here instead.
	if _, err := os.Stat(filepath.Join(dataDir, store.DBFile)); err != nil {
		return fmt.Errorf("no database in %s; start the API once to create it, then retry", dataDir)
	}
	db, err := store.Open(dataDir)
	if err != nil {
		return err
	}
	defer db.Close()
	ms, err := store.LoadMigrations(store.Migrations)
	if err != nil {
		return err
	}
	current, err := store.Version(db)
	if err != nil {
		return err
	}
	if current != len(ms) {
		return fmt.Errorf("database schema is version %d, want %d; start the API once to migrate it, then retry", current, len(ms))
	}
	err = auth.ResetPassword(db, username, password)
	if errors.Is(err, auth.ErrNotFound) {
		return fmt.Errorf("no admin named %q; nothing changed", username)
	}
	if err != nil {
		return err
	}
	slog.Info("password reset", "username", username)
	return nil
}
