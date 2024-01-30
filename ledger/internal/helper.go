package internal

import (
	"context"
	"database/sql"
	"errors"
)

func transact(ctx context.Context, db *sql.DB, txOptions *sql.TxOptions, fn func(ctx context.Context, tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, txOptions)
	if err != nil {
		return err
	}
	err = fn(ctx, tx)
	if err != nil {
		errRollback := tx.Rollback()
		if errRollback != nil {
			err = errors.Join(err, errRollback)
		}
		return err
	}
	return tx.Commit()
}
