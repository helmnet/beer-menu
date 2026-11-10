package importer

import (
	"database/sql"
	"testing"

	"github.com/helmnet/beer-menu/api/internal/secrets"
)

// OpenTestDB and NewTestBox expose the internal test helpers to the importer_test package,
// whose tests use importertest (which imports importer, so it cannot be used from here).
func OpenTestDB(t *testing.T) *sql.DB { return openDB(t) }

// NewTestBox returns a Box on a fresh random key.
func NewTestBox(t *testing.T) *secrets.Box { return newBox(t) }
