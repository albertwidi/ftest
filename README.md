# Ledger Service

Welcome to the ledger service, it's a place where you can store and transfer money from one account to another.

## NOTES

The program is expected to be running from a clean state, this means the container must be brought down first when something goes wrong. This is because some bootstrap steps will fail due duplicates.

To ensure this process, please use `make run` to run the program.

## Pre-Created Accounts

There are three accounts created when the program starts:

1. `test-fund` as a `funding` account.
2. `test-acc-1` as a user account. This account is funded with 10.000.
3. `test-acc-2` as a user account. This account is funded with 20.000.

## How To

To help the execution of below actions, we will use `make`. Please ensure you have `make` in your environment.

### Run

To run the program, please invoke `make run`.

It will first ensure the `Postgres` container is running via `docker-compose`. Then we will run the migration script to ensure all tables are present. Then we invoke `go run main.go`.

After the program running, you can start to hit the HTTP endpoints. At `:8080` by default.

### Test

To run the test, please invoke `make test`.

It will first ensure the `Postgres` container is running via `docker-compose`. Then we will run the migration script to ensure all tables are present. Then invoke `go test -v ./... -p=1`.

The `-p=1` is needed because we are doing integration test and we will be doing that sequentially.

### Applying Database Schema

To apply the database schema into the database, you can use `make migrate`. It will apply the schema that located inside `./database/ledger/schema.sql`.

To do this, we are not using `/docker-entrypoint-initdb.d`. Instead we use a simple one-liner script to apply the `schema.sql` to the database. The reason we don't do this is because we don't want the migration to be tightly coupled to the initialization of the container/container start. We want it to be a separate process so we don't have to tear down the container every time we want to make changes to our database schema.
