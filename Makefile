build:

run: compose migrate
	go run main.go

test: compose migrate
	go test -v -race ./... -p=1

compose:
	docker-compose up -d

migrate: compose
# Give the chance for the container to start first. Its a bit annoying if we use the command indivudually though.
	sleep 1
	PGPASSWORD=postgres docker exec -it $(shell docker ps -aqf "name=test-postgres") psql -U postgres -d ledger -f /data/ledger/schema.sql

.PHONY: run test migrate
