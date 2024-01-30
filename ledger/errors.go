package ledger

import "errors"

var (
	ErrInsufficientBalance        = errors.New("insufficient balance")
	ErrLedgerEntriesTotalNotZero  = errors.New("non zero sum of ledger entries")
	ErrInvalidLedgerEntriesLength = errors.New("ledger entries must have even length")
	ErrAllAccountsNotfound        = errors.New("all accounts not found")
	ErrAccountNotFound            = errors.New("account not found")
)
