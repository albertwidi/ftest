package ledger

import (
	"context"
	"database/sql"
	"log"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

var testLedger *Ledger

func TestMain(m *testing.M) {
	db, err := sql.Open("postgres", "postgres://postgres:postgres@localhost:5432/ledger?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	testLedger = New(db)

	os.Exit(m.Run())
}

func createFundingAccount(t *testing.T, ledger *Ledger) Account {
	if !testing.Testing() {
		panic("cannot create funding account in non-testing mode")
	}

	acc, err := ledger.CreateAccount(context.Background(), "", AccountTypeFunding)
	if err != nil {
		t.Fatal(err)
	}
	return acc
}
