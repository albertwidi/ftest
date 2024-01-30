package internal

import (
	"database/sql"
	"log"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

var testPG *Postgres

func TestMain(m *testing.M) {
	db, err := sql.Open("postgres", "postgres://postgres:postgres@localhost:5432/ledger?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	testPG = NewPostgres(db)

	os.Exit(m.Run())
}
