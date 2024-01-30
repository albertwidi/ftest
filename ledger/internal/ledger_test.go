package internal

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestCreateAccount(t *testing.T) {
	t.Cleanup(func() {
		TruncateTables(t, testPG, "accounts", "accounts_balance")
	})

	accountID := "acc-id"
	createdAt := time.Now()

	expectAccount := Account{
		ID:          accountID,
		AccountType: "user",
		CreatedAt:   createdAt,
	}
	expectAccountBalance := AccountBalance{
		AccountID:         accountID,
		CreatedAt:         createdAt,
		Balance:           decimal.NewFromInt(0),
		LastTransactionID: "",
	}

	if err := testPG.CreateAccount(context.Background(), Account{ID: accountID, AccountType: "user", CreatedAt: createdAt}); err != nil {
		t.Fatal(err)
	}

	account, err := testPG.GetAccount(context.Background(), accountID)
	if err != nil {
		t.Fatal(err)
	}
	accBal, err := testPG.GetAccountsBalance(context.Background(), accountID)
	if err != nil {
		t.Fatal(err)
	}
	if len(accBal) == 0 {
		t.Fatal("expecting 1 account balance created")
	}

	if diff := cmp.Diff(
		expectAccount,
		account,
		cmpopts.IgnoreFields(Account{}, "UpdatedAt"),
	); diff != "" {
		t.Fatalf("(-want/+got) Account:\n%s", diff)
	}
	if diff := cmp.Diff(
		expectAccountBalance,
		accBal[0],
		cmpopts.IgnoreFields(AccountBalance{}, "UpdatedAt"),
	); diff != "" {
		t.Fatalf("(-want/+got) AccountBalance:\n%s", diff)
	}
}

// TestGetAccounts test whether the get accounts behave as expected. For example:
// - It should not return random accounts.
// - It should not return error if one of the account is not exist.
func TestGetAccountsBalance(t *testing.T) {
	t.Cleanup(func() {
		TruncateTables(t, testPG, "accounts", "accounts_balance")
	})

	// Prepare the accounts for the test.
	accounts := []Account{
		{
			ID:                   "acc-1",
			AccountType:          "funding",
			AllowNegativeBalance: true,
			CreatedAt:            time.Now(),
		},
		{
			ID:                   "acc-2",
			AccountType:          "user",
			AllowNegativeBalance: false,
			CreatedAt:            time.Now(),
		},
		{
			ID:                   "acc-3",
			AccountType:          "user",
			AllowNegativeBalance: false,
			CreatedAt:            time.Now(),
		},
	}
	for _, account := range accounts {
		if err := testPG.CreateAccount(context.Background(), account); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name     string
		accounts []string
		expect   []AccountBalance
		err      error
	}{
		{
			name: "multiple valid accounts",
			accounts: []string{
				"acc-1",
				"acc-2",
				"acc-3",
			},
			expect: []AccountBalance{
				{
					AccountID:         "acc-1",
					AllowNegative:     true,
					Balance:           decimal.NewFromInt(0),
					LastTransactionID: "",
				},
				{
					AccountID:         "acc-2",
					AllowNegative:     false,
					Balance:           decimal.NewFromInt(0),
					LastTransactionID: "",
				},
				{
					AccountID:         "acc-3",
					AllowNegative:     false,
					Balance:           decimal.NewFromInt(0),
					LastTransactionID: "",
				},
			},
		},
		{
			name: "one invalid account",
			accounts: []string{
				"acc-1",
				"acc-2",
				"acc-4",
			},
			expect: []AccountBalance{
				{
					AccountID:         "acc-1",
					AllowNegative:     true,
					Balance:           decimal.NewFromInt(0),
					LastTransactionID: "",
				},
				{
					AccountID:         "acc-2",
					AllowNegative:     false,
					Balance:           decimal.NewFromInt(0),
					LastTransactionID: "",
				},
			},
		},
		{
			name: "all invalid accounts",
			accounts: []string{
				"acc-5",
				"acc-6",
				"acc-7",
			},
			expect: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			accounts, err := testPG.GetAccountsBalance(context.Background(), test.accounts...)
			if err != test.err {
				t.Fatalf("expecting error %v but got %v", test.err, err)
			}

			opts := []cmp.Option{
				cmpopts.IgnoreFields(AccountBalance{}, "CreatedAt", "UpdatedAt"),
			}
			if diff := cmp.Diff(test.expect, accounts, opts...); diff != "" {
				t.Fatalf("(-want/+got) AccountsBalance:\n%s", diff)
			}
		})
	}
}

// TestCorrectBalance tests whether we will get the correct balance even if we create transactions concurrently.
func TestCorrectBalance(t *testing.T) {
	t.Cleanup(func() {
		TruncateTables(t, testPG, "accounts_balance", "transaction", "accounts_ledger")
	})

	t.Run("balance should not negative", func(t *testing.T) {
		if err := transact(context.Background(), testPG.db, nil, func(ctx context.Context, tx *sql.Tx) error {
			return createAccountBalance(context.Background(), tx, AccountBalance{
				AccountID:     "one",
				Balance:       decimal.NewFromInt(100_000),
				AllowNegative: false,
				CreatedAt:     time.Now(),
			})
		}); err != nil {
			t.Fatal(err)
		}
		if err := transact(context.Background(), testPG.db, nil, func(ctx context.Context, tx *sql.Tx) error {
			return createAccountBalance(context.Background(), tx, AccountBalance{
				AccountID:     "two",
				Balance:       decimal.NewFromInt(0),
				AllowNegative: false,
				CreatedAt:     time.Now(),
			})
		}); err != nil {
			t.Fatal(err)
		}

		var txs []func() error
		var errs []error
		var errLock sync.Mutex
		wg := sync.WaitGroup{}

		// Create transaction of 3.000 each, and do 40 transactions concurrently with total of 120.000.
		for i := 0; i < 40; i++ {
			wg.Add(1)
			txs = append(txs, func() error {
				txID := uuid.NewString()
				err := testPG.CreateTransaction(context.Background(), CreateTransaction{
					TransactionID: uuid.NewString(),
					CreatedAt:     time.Now(),
					LedgerEntries: []Ledger{
						{
							TransactionID: txID,
							AccountID:     "one",
							Amount:        decimal.NewFromInt(-3_000),
							CreatedAt:     time.Now(),
						},
						{
							TransactionID: txID,
							AccountID:     "two",
							Amount:        decimal.NewFromInt(3_000),
							CreatedAt:     time.Now(),
						},
					},
					Summaries: map[string]decimal.Decimal{
						"one": decimal.NewFromInt(-3000),
						"two": decimal.NewFromInt(3000),
					},
				})
				if err != nil {
					errLock.Lock()
					errs = append(errs, err)
					errLock.Unlock()
				}
				wg.Done()
				return err
			})
		}

		for _, tx := range txs {
			go tx()
		}
		wg.Wait()

		balance, err := testPG.GetAccountsBalance(context.Background(), "one", "two")
		if err != nil {
			t.Fatal(err)
		}
		if balance[0].Balance.Cmp(decimal.NewFromInt(1000)) != 0 {
			t.Fatalf("expecting balance %d but got %s", 1000, balance[0].Balance.String())
		}
		if balance[1].Balance.Cmp(decimal.NewFromInt(99000)) != 0 {
			t.Fatalf("expecting balance %d but got %s", 99000, balance[1].Balance.String())
		}

		if len(errs) != 7 {
			t.Fatalf("expecting 6 erros but got %d", len(errs))
		}

		// Check ledger entries of 'one'. The total of the entries should be -99_000
		entries1, err := testPG.GetLedgerByAccountID(context.Background(), "one")
		if err != nil {
			t.Fatal(err)
		}
		total1 := decimal.Zero
		for _, entry := range entries1 {
			total1 = total1.Add(entry.Amount)
		}
		if total1.Cmp(decimal.NewFromInt(-99_000)) != 0 {
			t.Fatalf("one: expecting total of -99000 but got %s", total1.String())
		}

		// Check ledger entries of 'two'. The total of the entries should be -99_000
		entries2, err := testPG.GetLedgerByAccountID(context.Background(), "two")
		if err != nil {
			t.Fatal(err)
		}
		total2 := decimal.Zero
		for _, entry := range entries2 {
			total2 = total2.Add(entry.Amount)
		}
		if total2.Cmp(decimal.NewFromInt(99000)) != 0 {
			t.Fatalf("two: expecting total of 99000 but got %s", total2.String())
		}
	})
}
