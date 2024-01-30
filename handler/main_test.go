package handler

import (
	"database/sql"
	"log"
	"os"
	"testing"

	_ "github.com/lib/pq"

	"github.com/albertwidi/ftest/ledger"
)

var testHandler *Handler

func TestMain(m *testing.M) {
	db, err := sql.Open("postgres", "postgres://postgres:postgres@localhost:5432/ledger?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	ld := ledger.New(db)
	testHandler = New(ld)

	os.Exit(m.Run())
}
