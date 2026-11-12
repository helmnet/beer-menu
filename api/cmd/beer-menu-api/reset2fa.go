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

// resetTwoFactor runs "beer-menu-api reset-2fa <username>" (research R12), the operator's
// recovery when an admin has lost every factor and recovery code and no other admin can clear
// them. args are the words after the subcommand. It opens the existing database in dataDir,
// refuses unless the API has migrated it to the newest schema, then deletes that admin's
// factors, recovery codes, pending steps and sessions in one transaction and logs the username
// and time (FR-029). It leaves the require_2fa setting as it was. Errors go to out. It returns
// the exit code: 0 on success, 1 with nothing changed otherwise.
func resetTwoFactor(dataDir string, args []string, out io.Writer) int {
	if err := runResetTwoFactor(dataDir, args); err != nil {
		fmt.Fprintln(out, "reset-2fa:", err)
		return 1
	}
	fmt.Fprintln(out, "Two-factor sign-in cleared. All of that admin's sessions have ended.")
	return 0
}

func runResetTwoFactor(dataDir string, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: beer-menu-api reset-2fa <username>")
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
	admin, err := auth.FindAdmin(db, args[0])
	if err == nil {
		err = auth.ClearTwoFactor(db, admin.ID)
	}
	if errors.Is(err, auth.ErrNotFound) {
		return fmt.Errorf("no admin named %q; nothing changed", args[0])
	}
	if err != nil {
		return err
	}
	slog.Info("two-factor reset", "username", admin.Username)
	return nil
}
