package internal

import (
	"fmt"
	"strings"
	"testing"
)

// TruncateTables truncates list of tables passed in the parameter. This function is guarded by
// test check so it cannot be used outside of testing.
func TruncateTables(t *testing.T, pg *Postgres, tables ...string) {
	if !testing.Testing() {
		return
	}
	t.Helper()

	tabs := strings.Join(tables, ",")
	query := fmt.Sprintf("TRUNCATE %s RESTART IDENTITY;", tabs)

	_, err := pg.db.Exec(query)
	if err != nil {
		t.Log(err)
	}
}
