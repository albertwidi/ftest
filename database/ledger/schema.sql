-- drop tables.
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS transaction;
DROP TABLE IF EXISTS accounts_balance;
DROP TABLE IF EXISTS accounts_ledger;

-- types.
DROP TYPE IF EXISTS account_type;
CREATE TYPE account_type AS ENUM('user','funding');

-- accounts is used to store all user accounts.
CREATE TABLE IF NOT EXISTS accounts(
	"account_id" VARCHAR PRIMARY KEY,
	"account_type" account_type NOT NULL,
	"created_at" TIMESTAMPTZ NOT NULL,
	"updated_at" TIMESTAMPTZ
);

-- transaction is used to store all transaction records.
CREATE TABLE IF NOT EXISTS transaction(
	"transaction_id" VARCHAR PRIMARY KEY,
	"amount" NUMERIC NOT NULL,
	"created_at" TIMESTAMPTZ NOT NULL,
	"updated_at" TIMESTAMPTZ
);

-- accounts_balance is used to store the latest state of user's balance. This table will be used for user
-- balance fast retrieval and for locking the user balance for transaction.
CREATE TABLE IF NOT EXISTS accounts_balance(
	"account_id" VARCHAR PRIMARY KEY,
	-- allow_negative allows some accounts to have negative balance. For example, for the funding
	-- account we might allow the account to have negative balance.
	"allow_negative" BOOLEAN NOT NULL,
	"balance" NUMERIC NOT NULL,
	"last_transaction_id" VARCHAR NOT NULL,
	"created_at" TIMESTAMPTZ NOT NULL,
	"updated_at" TIMESTAMPTZ
);

-- accounts_ledger is used to store all ledger changes for a specific account. A single transaction
-- can possibly affecting multiple acounts in the ledger.
--
-- Row in this table is immutable and should not be updated.
CREATE TABLE IF NOT EXISTS accounts_ledger(
	"transaction_id" VARCHAR NOT NULL,
	"account_id" VARCHAR NOT NULL,
	"amount" NUMERIC NOT NULL,
	"current_balance" NUMERIC NOT NULL,
	"previous_balance" NUMERIC NOT NULL,
	"created_at" TIMESTAMPTZ NOT NULL,
	"timestamp" BIGINT NOT NULL,
	-- the primary key of accounts_ledger is a composite of 'transaction_id' and 'account_id'.
	-- This is because we are recording balance changes for different-different account in a single transaction.
	PRIMARY KEY("transaction_id", "account_id")
);
