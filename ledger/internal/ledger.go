package internal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/shopspring/decimal"
)

// ErrInsufficientBalance returned when user's balance is not enough to do the transaction. The only different with the error in the
// ledger package is this error located deeper in postgres layer. The benefit of this error is we can differentiate the location of the error.
var ErrInsufficientBalance = errors.New("account has insufficient balance")

type Account struct {
	// ID is the unique identifier for each account.
	ID          string
	AccountType string
	CreatedAt   time.Time
	UpdatedAt   sql.NullTime

	// AllowNegativeBalance is a special flag for account creation. This flag allows account balance
	// to be negative in some cases.
	AllowNegativeBalance bool
}

type AccountBalance struct {
	AccountID         string
	AllowNegative     bool
	Balance           decimal.Decimal
	LastTransactionID string
	CreatedAt         time.Time
	UpdatedAt         sql.NullTime
}

type Transaction struct {
	TransactionID   string
	TransactionType string
	Amount          string
	CreatedAt       time.Time
	UpdatedAt       sql.NullTime
}

// Ledger stores immutable records of money changes per account id.
//
// Every ledger record is unique per transaction_id and account_id.
type Ledger struct {
	TransactionID string
	AccountID     string
	// Amount is the amount of balance change.
	Amount decimal.Decimal
	// CurrentBalance is the final amount after balance change.
	CurrentBalance decimal.Decimal
	// PreviousAmount is the previous amount before balance change.
	PreviousBalance decimal.Decimal
	CreatedAt       time.Time
	Timestamp       int64
}

// CreateAccount creates a unique account for the user to allowed user to transact.
// In the creation of the account, we will also create the account's balance in account_balance table.
func (p *Postgres) CreateAccount(ctx context.Context, acc Account) error {
	query := "INSERT INTO accounts(account_id, account_type, created_at) VALUES($1,$2,$3);"

	return transact(ctx, p.db, nil, func(ctx context.Context, db *sql.Tx) error {
		_, err := db.Exec(query, acc.ID, acc.AccountType, acc.CreatedAt)
		if err != nil {
			return err
		}
		if err := createAccountBalance(ctx, db, AccountBalance{
			AccountID:         acc.ID,
			AllowNegative:     acc.AllowNegativeBalance,
			Balance:           decimal.Zero,
			LastTransactionID: "",
			CreatedAt:         acc.CreatedAt,
		}); err != nil {
			return err
		}
		return nil
	})
}

func createAccountBalance(ctx context.Context, db *sql.Tx, balance AccountBalance) error {
	query := `
		INSERT INTO accounts_balance(account_id, allow_negative, balance, last_transaction_id, created_at)
		VALUES($1,$2,$3,$4,$5);
	`
	_, err := db.Exec(query, balance.AccountID, balance.AllowNegative, balance.Balance, "", balance.CreatedAt)
	return err
}

// GetAccount returns account information.
func (p *Postgres) GetAccount(ctx context.Context, accountID string) (Account, error) {
	acc := Account{}
	query := "SELECT account_id, account_type, created_at, updated_at from accounts where account_id = $1;"
	row := p.db.QueryRow(query, accountID)
	err := row.Scan(
		&acc.ID,
		&acc.AccountType,
		&acc.CreatedAt,
		&acc.UpdatedAt,
	)
	return acc, err
}

// GetAccounts retrieves multiple accounts_balance if the accounts in parameter is exist. The function
// does not throw error if any one of the account is not available.
func (p *Postgres) GetAccountsBalance(ctx context.Context, accounts ...string) ([]AccountBalance, error) {
	query, params, err := squirrel.Select("account_id", "allow_negative", "balance", "last_transaction_id", "created_at", "updated_at").
		From("accounts_balance").
		Where(squirrel.Eq{"account_id": accounts}).
		PlaceholderFormat(squirrel.Dollar).
		ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := p.db.Query(query, params...)
	if err != nil {
		return nil, err
	}

	var accs []AccountBalance
	for rows.Next() {
		acc := AccountBalance{}
		if err := rows.Scan(
			&acc.AccountID,
			&acc.AllowNegative,
			&acc.Balance,
			&acc.LastTransactionID,
			&acc.CreatedAt,
			&acc.UpdatedAt,
		); err != nil {
			return nil, err
		}
		accs = append(accs, acc)
	}
	return accs, nil
}

type CreateTransaction struct {
	TransactionID string
	Amount        decimal.Decimal
	CreatedAt     time.Time
	// LedgerEntries is the entries within the transaction.
	LedgerEntries []Ledger
	// Summaries is the summary of the transaction per account. This means this is the total of DEBIT/CREDIT
	// per account basis. This information is needed as we will lock all the accounts listed here when doing
	// a transaction.
	Summaries map[string]decimal.Decimal
}

// CreateTransaction creates a new transaction and transfers money from one account to another
// depdends on the requirement of the ledger.
//
// The current implementation are doing this in one transaction with ReadCommitted transaction level:
// 1. SELECT FOR UPDATE all affected accounts.
// 2. Insert transaction record to transaction table.
// 3. Update all balances based on calculation of balance changes.
// 4. Insert all ledger entries records.
func (p *Postgres) CreateTransaction(ctx context.Context, tx CreateTransaction) error {
	counter := 0
	accountIDs := make([]string, len(tx.Summaries))
	for accID := range tx.Summaries {
		accountIDs[counter] = "'" + accID + "'"
		counter++
	}
	accountIDsForQuery := strings.Join(accountIDs, ",")

	// selectForUpdateQuery is used to lock all accounts_balance listed in the transaction summaries. This is to ensure
	// the balance is not changing while we are doing a transaction.
	//
	// Please NOTE that select for update is only works inside a TRANSACTION.
	selectForUpdateQuery := fmt.Sprintf(`
		SELECT account_id, balance, allow_negative
		FROM accounts_balance
		WHERE account_id IN(%s)
		FOR UPDATE;
	`, accountIDsForQuery)

	// insertTransactionQuery inserts new transaction to the record.
	insertTransactionQuery := "INSERT INTO transaction(transaction_id, amount, created_at) VALUES($1,$2,$3)"

	// updateBalanceQuery updates multiple account balances with updated balance on each account.
	updateBalanceQuery := `
		UPDATE accounts_balance AS ab SET
			balance = v.balance,
			last_transaction_id = v.transaction_id,
			updated_at = v.updated_at
		FROM (VALUES %s) AS v(balance, transaction_id, account_id, updated_at)
		WHERE ab.account_id = v.account_id;
	`

	// insertLedgerBuilder inserts multiple ledger records for affected accounts.
	insertLedgerBuilder := squirrel.Insert("accounts_ledger").Columns("transaction_id", "account_id", "amount", "current_balance", "previous_balance", "created_at", "timestamp")
	// maps all the ledger entries to each account.
	ledgerMap := make(map[string][]Ledger)
	for _, ledger := range tx.LedgerEntries {
		ledgerMap[ledger.AccountID] = append(ledgerMap[ledger.AccountID], ledger)
	}

	return transact(ctx, p.db, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	}, func(ctx context.Context, db *sql.Tx) error {
		var updateQuery string

		// Do SELECT FOR UPDATE to ensure we are locking the balance first.
		rows, err := db.Query(selectForUpdateQuery)
		if err != nil {
			return fmt.Errorf("failed to lock accounts with error: %v", err)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		for rows.Next() {
			balance := AccountBalance{}
			if err := rows.Scan(
				&balance.AccountID,
				&balance.Balance,
				&balance.AllowNegative,
			); err != nil {
				return err
			}
			toBalance := balance.Balance.Add(tx.Summaries[balance.AccountID])
			// Check the balance of the accounts again, as there might be a gap from where select the balance previously
			// up to this point where we select the balance for update.
			if toBalance.LessThan(decimal.Zero) && !balance.AllowNegative {
				return ErrInsufficientBalance
			}
			// Append the update query with ($1,$2,$3,$4) of to_balance, transaction_id, account_id, updated_at.
			updateQuery = updateQuery + fmt.Sprintf("(%s, '%s', '%s', to_timestamp(%d)),", toBalance.String(), tx.TransactionID, balance.AccountID, tx.CreatedAt.Unix())

			// Set the previous and current balance to the retrieved balance. We will change this variables to reflect
			// the balance changes in the ledger.
			previousBalance := balance.Balance
			currentBalance := balance.Balance
			// Loop through all the ledgers for the account to calculate the current_balance and the previous_balance. This is important because in
			// one transaction, there might be multiple records on the same account. For example, transfering balance from one account to multiple accounts.
			ledgers := ledgerMap[balance.AccountID]
			for _, ledger := range ledgers {
				// Set the current balance to current_balance + amount.
				currentBalance = currentBalance.Add(ledger.Amount)
				insertLedgerBuilder = insertLedgerBuilder.Values(
					tx.TransactionID,
					ledger.AccountID,
					ledger.Amount,
					currentBalance,
					previousBalance,
					ledger.CreatedAt,
					ledger.CreatedAt.UnixNano(),
				)
				// Set the previous balance with the current balance as we have record the previous balance.
				previousBalance = currentBalance
			}
		}

		// Trim the last "," from the update query. Because it is basically VALUES(($1),($2),$(3)).
		updateQuery = strings.TrimSuffix(updateQuery, ",")
		updateBalanceQuery = fmt.Sprintf(updateBalanceQuery, updateQuery)

		// Insert the transaction record.
		_, err = db.Exec(insertTransactionQuery, tx.TransactionID, tx.Amount, tx.CreatedAt)
		if err != nil {
			return fmt.Errorf("failed insert new transaction with error: %v", err)
		}

		// Update the balance of the accounts.
		_, err = db.Exec(updateBalanceQuery)
		if err != nil {
			return fmt.Errorf("failed to update balances with error: %v", err)
		}

		// Insert all ledger entries.
		insertLedgerQuery, args, err := insertLedgerBuilder.PlaceholderFormat(squirrel.Dollar).ToSql()
		if err != nil {
			return fmt.Errorf("failed to build query for ledger entries with error: %v", err)
		}
		_, err = db.Exec(insertLedgerQuery, args...)
		if err != nil {
			return fmt.Errorf("failed to insert ledger entries with error: %v", err)
		}
		return nil
	})
}

func (p *Postgres) GetLedgerByAccountID(ctx context.Context, accountID string) ([]Ledger, error) {
	query := `
		SELECT transaction_id, account_id, amount, current_balance, previous_balance, created_at, timestamp
		FROM accounts_ledger
		WHERE account_id = $1
		ORDER BY timestamp ASC;
	`

	rows, err := p.db.Query(query, accountID)
	if err != nil {
		return nil, err
	}
	var entries []Ledger
	for rows.Next() {
		ledger := Ledger{}
		if err := rows.Scan(
			&ledger.TransactionID,
			&ledger.AccountID,
			&ledger.Amount,
			&ledger.CurrentBalance,
			&ledger.PreviousBalance,
			&ledger.CreatedAt,
			&ledger.Timestamp,
		); err != nil {
			return nil, err
		}
		entries = append(entries, ledger)
	}
	return entries, nil
}

func (p *Postgres) GetLedgerByTransactionID(ctx context.Context, transactionID string) ([]Ledger, error) {
	query := `
		SELECT transaction_id, account_id, amount, current_balance, previous_balance, created_at, timestamp
		FROM accounts_ledger
		WHERE transaction_id = $1
		ORDER BY timestamp ASC;
	`

	rows, err := p.db.Query(query, transactionID)
	if err != nil {
		return nil, err
	}
	var entries []Ledger
	for rows.Next() {
		ledger := Ledger{}
		if err := rows.Scan(
			&ledger.TransactionID,
			&ledger.AccountID,
			&ledger.Amount,
			&ledger.CurrentBalance,
			&ledger.PreviousBalance,
			&ledger.CreatedAt,
			&ledger.Timestamp,
		); err != nil {
			return nil, err
		}
		entries = append(entries, ledger)
	}
	return entries, nil
}
