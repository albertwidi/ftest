package ledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/albertwidi/ftest/ledger/internal"
)

const (
	AccountTypeUser    = "user"
	AccountTypeFunding = "funding"
)

// TransactionBuildeer is an interface to define what type can build a transaction. The interface is a bit unique
// because its implemented where it being defined because only want to use the interface internally to generalize
// the builder type.
type TransactionBuilder interface {
	validate() error
	buildTransaction(transactionID string) internal.CreateTransaction
}

// txSummaries is the summaries of the transaction per account. It contains the SUM of DEBIT/CREDIT amount.
type txSumaries map[string]decimal.Decimal

// txSummaries returns all accounts in a form of string array.
func (sums txSumaries) accounts() []string {
	idx := 0
	accounts := make([]string, len(sums))

	for accID := range sums {
		accounts[idx] = accID
		idx++
	}
	return accounts
}

type Ledger struct {
	pg *internal.Postgres
}

// New creates new ledger object to interact with ledger service.
func New(db *sql.DB) *Ledger {
	return &Ledger{
		pg: internal.NewPostgres(db),
	}
}

type Account struct {
	ID                   string
	AccountType          string
	AllowNegativeBalance bool
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (l *Ledger) CreateAccount(ctx context.Context, accountID, accountType string) (Account, error) {
	var allowNegative bool
	if accountType == "" {
		accountType = AccountTypeUser
	}
	if accountType == AccountTypeFunding {
		allowNegative = true
	}

	if accountID == "" {
		accountID = uuid.NewString()
	} else if len(accountID) > 10 {
		// For self creation, we forbid ID that longer than 10.
		return Account{}, errors.New("self account creation cannot have more than 10 character")
	}
	createdAt := time.Now()
	err := l.pg.CreateAccount(ctx, internal.Account{
		ID:                   accountID,
		AccountType:          accountType,
		AllowNegativeBalance: allowNegative,
		CreatedAt:            createdAt,
	})
	if err != nil {
		return Account{}, err
	}

	return Account{
		ID:                   accountID,
		AccountType:          accountType,
		AllowNegativeBalance: allowNegative,
		CreatedAt:            createdAt,
		UpdatedAt:            createdAt,
	}, nil
}

// AccountBalance stores the information of the account balance. This representation is different from the database layer
// as we might have some informations stripped or we want to use struct tag in the database layer.
type AccountBalance struct {
	AccountID         string
	Balance           decimal.Decimal
	AllowNegative     bool
	LastTransactionID string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// GetAccountBalance returns account balance by passing account_id.
func (l *Ledger) GetAccountBalance(ctx context.Context, accountID string) (AccountBalance, error) {
	balances, err := l.pg.GetAccountsBalance(ctx, accountID)
	if err != nil {
		return AccountBalance{}, err
	}
	if len(balances) == 0 {
		return AccountBalance{}, ErrAccountNotFound
	}
	return AccountBalance{
		AccountID:         balances[0].AccountID,
		Balance:           balances[0].Balance,
		AllowNegative:     balances[0].AllowNegative,
		LastTransactionID: balances[0].LastTransactionID,
		CreatedAt:         balances[0].CreatedAt,
		UpdatedAt:         balances[0].UpdatedAt.Time,
	}, nil
}

type LedgerEntry struct {
	TransactionID   string
	AccountID       string
	Amount          decimal.Decimal
	CurrentBalance  decimal.Decimal
	PreviousBalance decimal.Decimal
	CreatedAt       time.Time
}

func (l *Ledger) GetAccountLedgerEntries(ctx context.Context, accountID string) ([]LedgerEntry, error) {
	entries, err := l.pg.GetLedgerByAccountID(ctx, accountID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}

	le := make([]LedgerEntry, len(entries))
	for idx, entry := range entries {
		le[idx] = LedgerEntry{
			TransactionID:   entry.TransactionID,
			AccountID:       entry.AccountID,
			Amount:          entry.Amount,
			CurrentBalance:  entry.CurrentBalance,
			PreviousBalance: entry.PreviousBalance,
			CreatedAt:       entry.CreatedAt,
		}
	}
	return le, nil
}

// checkBalances retrieves all accounts balance information and do checks on them. This function checks two things:
// 1. Whether the account is already created or not.
// 2. Whether the account that doing transaction have enough money or not.
func (l *Ledger) checkBalances(ctx context.Context, summaries txSumaries) error {
	accounts := summaries.accounts()
	// GetAccountsBalance also acts as checking whether the account is present or not.
	balances, err := l.pg.GetAccountsBalance(ctx, accounts...)
	if err != nil {
		return err
	}
	if len(balances) == 0 {
		return ErrAllAccountsNotfound
	}

	// We are finding the account_id inside of the list of accounts balance with loop inside loop O(n^2). This should
	// be fine if n is small. And for this case, the n is small.
	for accID, sum := range summaries {
		var found bool
		for _, balance := range balances {
			if accID == balance.AccountID {
				found = true

				// Check for the balance and if it goes negative, check whether the account can go below zero(0).
				// We allow some accounts to go below 0, for example the account to fund user's money.
				if sum.Add(balance.Balance).LessThan(decimal.Zero) && !balance.AllowNegative {
					return fmt.Errorf("%w: account_id %s doesn't have enough balance for this transaction", ErrInsufficientBalance, accID)
				}
				// Break the loop as we already found the account_id.
				break
			}
		}
		if !found {
			return fmt.Errorf("account_id %s not found", accID)
		}
	}
	return nil
}

// buildTransaction creates a transaction for the database layer, and it validates the builder via validate function.
// The function also checks whether the SUM of the ledger entries is 0. This is important because the final value of the
// ledger should be zero(as we are doing double entry bookeeping).
func buildTransaction(transactionID string, builder TransactionBuilder) (internal.CreateTransaction, error) {
	if err := builder.validate(); err != nil {
		return internal.CreateTransaction{}, err
	}

	tx := builder.buildTransaction(transactionID)
	// txSummaries is the total DEBIT/CREDIT of money per account. This information will be used
	// later to pre-check the balance availability of the account.
	txSummaries := make(txSumaries)

	ledgerEntriesLen := len(tx.LedgerEntries)
	if ledgerEntriesLen == 0 || ledgerEntriesLen%2 != 0 {
		return internal.CreateTransaction{}, ErrInvalidLedgerEntriesLength
	}

	// Validate the transaction as we need to check whether the total amount of transaction
	// is the same with the amount of the ledger. At the end of the day, the ledger SUM amount
	// should be 0.
	sum := decimal.NewFromInt(0)
	for _, ledger := range tx.LedgerEntries {
		sum = sum.Add(ledger.Amount)
		txSummaries[ledger.AccountID] = txSummaries[ledger.AccountID].Add(ledger.Amount)
	}
	if !sum.IsZero() {
		return internal.CreateTransaction{}, ErrLedgerEntriesTotalNotZero
	}
	tx.Summaries = txSummaries
	return tx, nil
}
