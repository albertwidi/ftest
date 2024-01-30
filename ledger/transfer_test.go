package ledger

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/shopspring/decimal"

	"github.com/albertwidi/ftest/ledger/internal"
)

// TestTransfer tests transfer between accounts and its ledger validity.
func TestTransfer(t *testing.T) {
	// Create four(4) different accounts. We will cover these cases:
	// 1. 'one' transfer to 'two'.
	// 2. 'two' transfer to 'three'.
	// 3. 'three' transfer to 'four'.
	// 4. 'four' transfer to 'one'.

	fundingAccount := createFundingAccount(t, testLedger)
	acc1, err := testLedger.CreateAccount(context.Background(), "", AccountTypeUser)
	if err != nil {
		t.Fatal(err)
	}
	acc2, err := testLedger.CreateAccount(context.Background(), "", AccountTypeUser)
	if err != nil {
		t.Fatal(err)
	}
	acc3, err := testLedger.CreateAccount(context.Background(), "", AccountTypeUser)
	if err != nil {
		t.Fatal(err)
	}
	acc4, err := testLedger.CreateAccount(context.Background(), "", AccountTypeUser)
	if err != nil {
		t.Fatal(err)
	}

	_, err = testLedger.Transfer(
		context.Background(),
		Transfer{
			FromAccount: fundingAccount.ID,
			ToAccount:   acc1.ID,
			Amount:      createDecimalFromString("100"),
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	transfers := []Transfer{
		{
			FromAccount: acc1.ID,
			ToAccount:   acc2.ID,
			Amount:      createDecimalFromString("100"),
		},
		{
			FromAccount: acc2.ID,
			ToAccount:   acc3.ID,
			Amount:      createDecimalFromString("100"),
		},
		{
			FromAccount: acc3.ID,
			ToAccount:   acc4.ID,
			Amount:      createDecimalFromString("100"),
		},
		{
			FromAccount: acc4.ID,
			ToAccount:   acc1.ID,
			Amount:      createDecimalFromString("100"),
		},
	}
	for _, tf := range transfers {
		transferAndCheck(t, tf)
	}
	expectEntries := []internal.Ledger{
		{
			AccountID:       acc1.ID,
			Amount:          createDecimalFromString("100"),
			PreviousBalance: createDecimalFromString("0"),
			CurrentBalance:  createDecimalFromString("100"),
		},
		{
			AccountID:       acc1.ID,
			Amount:          createDecimalFromString("-100"),
			PreviousBalance: createDecimalFromString("100"),
			CurrentBalance:  createDecimalFromString("0"),
		},
		{
			AccountID:       acc1.ID,
			Amount:          createDecimalFromString("100"),
			PreviousBalance: createDecimalFromString("0"),
			CurrentBalance:  createDecimalFromString("100"),
		},
	}
	entries, err := testLedger.pg.GetLedgerByAccountID(context.Background(), acc1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(expectEntries, entries, cmpopts.IgnoreFields(
		internal.Ledger{}, "TransactionID", "CreatedAt", "Timestamp",
	)); diff != "" {
		t.Fatalf("(-want/+got)\n%s", diff)
	}
}

func transferAndCheck(t *testing.T, transfer Transfer) {
	t.Helper()

	previousBalances, err := testLedger.pg.GetAccountsBalance(context.Background(), transfer.FromAccount, transfer.ToAccount)
	if err != nil {
		t.Fatal(err)
	}
	mapPrevious := make(map[string]internal.AccountBalance)
	for _, balance := range previousBalances {
		mapPrevious[balance.AccountID] = balance
	}

	_, err = testLedger.Transfer(
		context.Background(),
		transfer,
	)
	if err != nil {
		t.Fatal(err)
	}
	balances, err := testLedger.pg.GetAccountsBalance(context.Background(), transfer.FromAccount, transfer.ToAccount)
	if err != nil {
		t.Fatal(err)
	}
	mapBalances := make(map[string]internal.AccountBalance)
	for _, balance := range balances {
		mapBalances[balance.AccountID] = balance
	}

	expectFrom := mapPrevious[transfer.FromAccount].Balance.Add(transfer.Amount.Mul(decimal.NewFromInt(-1)))
	if expectFrom.Cmp(mapBalances[transfer.FromAccount].Balance) != 0 {
		t.Fatalf("expecting from balance %s but got %s", expectFrom.String(), mapBalances[transfer.FromAccount].Balance.String())
	}
	expectTo := mapPrevious[transfer.ToAccount].Balance.Add(transfer.Amount)
	if expectTo.Cmp(mapBalances[transfer.ToAccount].Balance) != 0 {
		t.Fatalf("expecting to balance %s but got %s", expectTo.String(), mapBalances[transfer.ToAccount].Balance.String())
	}
}
