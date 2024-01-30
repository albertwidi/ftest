package ledger

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/shopspring/decimal"

	"github.com/albertwidi/ftest/ledger/internal"
)

func TestBuildTransaction(t *testing.T) {
	tests := []struct {
		name          string
		transactionID string
		builder       TransactionBuilder
		expectTx      internal.CreateTransaction
		err           error
	}{
		{
			name:          "transfer transaction",
			transactionID: "tx-id-1",
			builder: Transfer{
				FromAccount: "acc-1",
				ToAccount:   "acc-2",
				Amount:      createDecimalFromString("10"),
			},
			expectTx: internal.CreateTransaction{
				TransactionID: "tx-id-1",
				Amount:        createDecimalFromString("10"),
				LedgerEntries: []internal.Ledger{
					{
						AccountID: "acc-1",
						Amount:    createDecimalFromString("-10"),
					},
					{
						AccountID: "acc-2",
						Amount:    createDecimalFromString("10"),
					},
				},
				Summaries: map[string]decimal.Decimal{
					"acc-1": createDecimalFromString("-10"),
					"acc-2": createDecimalFromString("10"),
				},
			},
			err: nil,
		},
		{
			name:          "invalid ledger entries",
			transactionID: "tx-id-1",
			builder: invalidLedgerEntries{
				FromAccount: "acc-1",
				ToAccount:   "acc-2",
				Amount:      createDecimalFromString("10"),
			},
			err: ErrLedgerEntriesTotalNotZero,
		},
		{
			name:          "invalid ledger entries length",
			transactionID: "tx-id-1",
			builder: invalidLedgerEntriesLength{
				FromAccount: "acc-1",
				ToAccount:   "acc-2",
				Amount:      createDecimalFromString("10"),
			},
			err: ErrInvalidLedgerEntriesLength,
		},
	}

	for _, test := range tests {
		tt := test
		t.Run(test.name, func(t *testing.T) {
			tx, err := buildTransaction(tt.transactionID, tt.builder)
			if err != tt.err {
				t.Fatalf("expecting error %v but got %v", tt.err, err)
			}

			opts := []cmp.Option{
				cmpopts.IgnoreFields(internal.CreateTransaction{}, "CreatedAt"),
				cmpopts.IgnoreFields(internal.Ledger{}, "CreatedAt"),
			}
			if diff := cmp.Diff(tt.expectTx, tx, opts...); diff != "" {
				t.Fatalf("(-want/+got) CreateTransaction:\n%s", diff)
			}
		})
	}
}

func TestCheckBalance(t *testing.T) {
	t.Cleanup(func() {
		internal.TruncateTables(
			t, testLedger.pg,
			"accounts", "accounts_balance", "transaction", "accounts_ledger",
		)
	})
	fundingAccount := createFundingAccount(t, testLedger)

	t.Run("both have sufficient balance", func(t *testing.T) {
		acc1, err := testLedger.CreateAccount(context.Background(), "", AccountTypeUser)
		if err != nil {
			t.Fatal(err)
		}
		acc2, err := testLedger.CreateAccount(context.Background(), "", AccountTypeUser)
		if err != nil {
			t.Fatal(err)
		}

		// Funds money to the account that want to transact. Since we want to transfer from 'acc1' to 'acc2'
		// then we only need to fund the 'acc1'.
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

		if err := testLedger.checkBalances(context.Background(), map[string]decimal.Decimal{
			acc1.ID: createDecimalFromString("-100"),
			acc2.ID: createDecimalFromString("100"),
		}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("no account found", func(t *testing.T) {
		err := testLedger.checkBalances(context.Background(), map[string]decimal.Decimal{
			"one": decimal.Zero,
			"two": decimal.Zero,
		})
		if err != ErrAllAccountsNotfound {
			t.Fatalf("expecing error %v but got %v", ErrAllAccountsNotfound, err)
		}
	})

	t.Run("insufficient balance", func(t *testing.T) {
		acc1, err := testLedger.CreateAccount(context.Background(), "", AccountTypeUser)
		if err != nil {
			t.Fatal(err)
		}
		acc2, err := testLedger.CreateAccount(context.Background(), "", AccountTypeUser)
		if err != nil {
			t.Fatal(err)
		}

		// Funds money to the account that want to transact. Since we want to transfer from 'acc1' to 'acc2'
		// then we only need to fund the 'acc1'.
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

		if err := testLedger.checkBalances(context.Background(), map[string]decimal.Decimal{
			acc1.ID: createDecimalFromString("-200"),
			acc2.ID: createDecimalFromString("100"),
		}); !errors.Is(err, ErrInsufficientBalance) {
			t.Fatalf("expecting error %v but got %v", ErrInsufficientBalance, err)
		}
	})
}

func createDecimalFromString(amount string) decimal.Decimal {
	d, err := decimal.NewFromString(amount)
	if err != nil {
		panic(err)
	}
	return d
}

type invalidLedgerEntries struct {
	FromAccount string
	ToAccount   string
	Amount      decimal.Decimal
}

func (i invalidLedgerEntries) validate() error { return nil }

func (i invalidLedgerEntries) buildTransaction(transactionID string) internal.CreateTransaction {
	return internal.CreateTransaction{
		TransactionID: transactionID,
		Amount:        i.Amount,
		CreatedAt:     time.Now(),
		LedgerEntries: []internal.Ledger{
			{
				AccountID: i.FromAccount,
				Amount:    i.Amount,
			},
			{
				AccountID: i.ToAccount,
				Amount:    i.Amount,
			},
		},
	}
}

type invalidLedgerEntriesLength struct {
	FromAccount string
	ToAccount   string
	Amount      decimal.Decimal
}

func (i invalidLedgerEntriesLength) validate() error { return nil }

func (i invalidLedgerEntriesLength) buildTransaction(transactionID string) internal.CreateTransaction {
	return internal.CreateTransaction{
		TransactionID: transactionID,
		Amount:        i.Amount,
		CreatedAt:     time.Now(),
		LedgerEntries: []internal.Ledger{
			{
				AccountID: i.FromAccount,
				Amount:    i.Amount,
			},
		},
	}
}
