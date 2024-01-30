run:
	docker compose down --remove-orphans
	docker compose --profile run up --force-recreate -d
	sleep 7
	./bootstrap.sh

test: compose migrate
	go test -v -race ./... -p=1

compose:
	docker compose down --remove-orphans
	docker compose --profile test up -d 

migrate: compose
# Give the chance for the container to start first. Its a bit annoying if we use the command indivudually though.
	sleep 1
	PGPASSWORD=postgres docker exec -it $(shell docker ps -aqf "name=test-postgres") psql -U postgres -d ledger -f /data/ledger/schema.sql

.PHONY: run test migrate
