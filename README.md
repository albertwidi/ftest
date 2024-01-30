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

These accounts and funding are placed inside the [bootstrap.sh](./bootstrap.sh). It placed outside of the program so we can scale the program easily and invoke the scripts once everything is up and running.

## How To

To help the execution of below actions, we will use `make`. Please ensure you have `make` in your environment.

### Run

To run the program, please invoke `make run`.

It will invoke `docker-compose up` and build the service using multi-stage build. We will also apply the schema inside the `docker-compose`.

After the program running, you can start to hit the HTTP endpoints. At `:8080` by default.

### Test

To run the test, please invoke `make test`.

It will first ensure the `Postgres` container is running via `docker-compose`. Then we will run the migration script to ensure all tables are present. Then invoke `go test -v ./... -p=1`.

The `-p=1` is needed because we are doing integration test and we will be doing that sequentially.

### Applying Database Schema

To apply the database schema into the database, you can use `make migrate`.

It will apply the schema that located inside `./database/ledger/schema.sql`.

To do this, we are not using `/docker-entrypoint-initdb.d`. Instead we use a simple one-liner script to apply the `schema.sql` to the database. The reason we don't do this is because we don't want the migration to be tightly coupled to the initialization of the container/container start. We want it to be a separate process so we don't have to tear down the container every time we want to make changes to our database schema.

## Example

This is an example of curling the endpoints.

1. Transfer [`POST /v1/ledger/transfer`]

	```shell
	❯ curl -s -X POST localhost:8080/v1/ledger/transfer -d '{"from_account": "test-fund", "to_account": "test-acc-1", "amount": "100"}' | jq

	{
		"transaction_id": "a2b695c0-3a7c-479c-9d2c-868c9eb78fe4"
	}
	```

1. Get Balance [`GET /v/1/ledger/balance`]

	```shell
	❯ curl -s 'localhost:8080/v1/ledger/balance?account_id=test-acc-1' | jq

	{
		"account_id": "test-acc-1",
		"available_balance": "11120.82",
		"last_updated": "2024-01-30 09:06:13 +0000 UTC"
	}
	```

1. Transaction List [`GET /v1/ledger`]

	```shell
	❯ curl -s 'localhost:8080/v1/ledger?account_id=test-acc-1' | jq

	{
		"transactions": [
			{
			"transaction_id": "3b672251-77b3-49f9-9916-e7354f135491",
			"account_id": "test-acc-1",
			"amount": "10000",
			"created_at": "2024-01-30 09:05:54.930281 +0000 UTC"
			},
			{
			"transaction_id": "5557bea1-cdc8-44bd-a72d-ac95a8716e57",
			"account_id": "test-acc-1",
			"amount": "20.235",
			"created_at": "2024-01-30 09:05:58.703959 +0000 UTC"
			},
			{
			"transaction_id": "17e77d86-af10-4fb6-a6b4-5149d189d020",
			"account_id": "test-acc-1",
			"amount": "100.485",
			"created_at": "2024-01-30 09:06:06.121206 +0000 UTC"
			},
			{
			"transaction_id": "b004129b-d17c-407f-9ae5-b2cef8510dfc",
			"account_id": "test-acc-1",
			"amount": "1000.1",
			"created_at": "2024-01-30 09:06:13.744738 +0000 UTC"
			}
		]
	}
	```

## Scaling

To scale the application, `replica` in `docker compose` is used and all the requests all load-balanced by `envoy-proxy` via port `8080`.

In this example, we are scaling the application to three(3) container.