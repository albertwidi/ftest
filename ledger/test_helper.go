package ledger

import (
	"context"
	"testing"

	"github.com/albertwidi/ftest/ledger/internal"
)

func BootstrapTest(t *testing.T, ld *Ledger) {
	if !testing.Testing() {
		panic("cannot use bootstrap test outside of go test")
	}
	t.Helper()

	_, err := ld.CreateAccount(context.Background(), "b-fund", AccountTypeFunding)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ld.CreateAccount(context.Background(), "b-acc-1", AccountTypeUser)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ld.CreateAccount(context.Background(), "b-acc-2", AccountTypeUser)
	if err != nil {
		t.Fatal(err)
	}
}

func RestLedger(t *testing.T, ld *Ledger) {
	if !testing.Testing() {
		panic("cannot use bootstrap test outside of go test")
	}
	t.Helper()

	internal.TruncateTables(t, ld.pg, []string{
		"transaction",
		"accounts",
		"accounts_balance",
		"accounts_ledger",
	}...)
}
