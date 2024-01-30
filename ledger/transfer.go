package ledger

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/albertwidi/ftest/ledger/internal"
)

type Transfer struct {
	FromAccount string
	ToAccount   string
	Amount      decimal.Decimal
}

func (t Transfer) validate() error {
	if t.FromAccount == "" {
		return errors.New("from account cannot be empty")
	}
	if t.ToAccount == "" {
		return errors.New("to account cannot be empty")
	}
	if t.Amount.IsZero() {
		return errors.New("amount cannot be zero/empty")
	}
	return nil
}

func (t Transfer) buildTransaction(transactionID string) internal.CreateTransaction {
	txTime := time.Now()
	tx := internal.CreateTransaction{
		TransactionID: transactionID,
		Amount:        t.Amount,
		CreatedAt:     txTime,
		LedgerEntries: []internal.Ledger{
			// Create the first entry of DEBIT to dedcut user's money.
			{
				AccountID: t.FromAccount,
				Amount:    t.Amount.Mul(decimal.NewFromInt(-1)),
				CreatedAt: txTime,
			},
			// Create the second etry of CREDIT to add user's money.
			{
				AccountID: t.ToAccount,
				Amount:    t.Amount,
				CreatedAt: txTime,
			},
		},
	}
	return tx
}

func (l *Ledger) Transfer(ctx context.Context, request Transfer) (string, error) {
	txID := uuid.NewString()
	tx, err := buildTransaction(txID, request)
	if err != nil {
		return txID, err
	}
	if err := l.checkBalances(ctx, tx.Summaries); err != nil {
		return txID, err
	}
	err = l.pg.CreateTransaction(ctx, tx)
	return txID, err
}
