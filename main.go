package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/lib/pq"

	"github.com/shopspring/decimal"

	"github.com/albertwidi/ftest/handler"
	"github.com/albertwidi/ftest/ledger"
	"github.com/go-chi/chi/v5"
)

type config struct {
	postgresDSN string
	servicePort string
}

func loadConfig() config {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/ledger?sslmode=disable"
	}
	servicePort := os.Getenv("SERVICE_PORT")
	if servicePort == "" {
		servicePort = "8080"
	}
	return config{
		postgresDSN: dsn,
		servicePort: servicePort,
	}
}

func main() {
	config := loadConfig()

	ctxSignal, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer cancel()

	db, err := sql.Open("postgres", config.postgresDSN)
	if err != nil {
		panic(err)
	}

	ld := ledger.New(db)
	bootstrap(ctxSignal, ld)

	r := chi.NewRouter()
	handle(ld, r)

	listener, err := net.Listen("tcp", ":"+config.servicePort)
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	errC := make(chan error, 1)
	server := &http.Server{
		Handler: r,
	}
	go func() {
		slog.Info(fmt.Sprintf("listening to %s", listener.Addr().String()))
		errC <- server.Serve(listener)
	}()

	select {
	case <-ctxSignal.Done():
	case err := <-errC:
		if err != nil {
			panic(err)
		}
	}
	slog.Info("ledger service shutdown")
}

// bootstrap bootstraps the program by creating several accounts for testing purposes.
//
// The bootstrap functions creates these accounts:
// 1. test-fund as an account that can fund another account.
// 2. test-acc-1 as a user account.
// 3. test-acc-2 as a user account.
//
// And also funds these following account with some balance:
// 1. test-acc-1 with 10_000.
// 2. test-acc-2 with 20_000.
func bootstrap(ctx context.Context, ld *ledger.Ledger) {
	slog.Info("creating test-fund account")
	_, err := ld.CreateAccount(ctx, "test-fund", ledger.AccountTypeFunding)
	if err != nil {
		panic(err)
	}
	slog.Info("creating test-acc-1 account")
	_, err = ld.CreateAccount(ctx, "test-acc-1", ledger.AccountTypeUser)
	if err != nil {
		panic(err)
	}
	slog.Info("creating test-acc-2 account")
	_, err = ld.CreateAccount(ctx, "test-acc-2", ledger.AccountTypeUser)
	if err != nil {
		panic(err)
	}

	slog.Info("funding test-acc-1 account")
	_, err = ld.Transfer(ctx, ledger.Transfer{
		FromAccount: "test-fund",
		ToAccount:   "test-acc-1",
		Amount:      decimal.NewFromInt(10_000),
	})
	if err != nil {
		panic(err)
	}
	slog.Info("funding test-acc-2 account")
	_, err = ld.Transfer(ctx, ledger.Transfer{
		FromAccount: "test-fund",
		ToAccount:   "test-acc-2",
		Amount:      decimal.NewFromInt(20_000),
	})
	if err != nil {
		panic(err)
	}
}

func handle(ld *ledger.Ledger, r chi.Router) {
	handler := handler.New(ld)
	r.Route("/v1/ledger", func(r chi.Router) {
		r.Post("/transfer", handler.LedgerTransfer)
		r.Get("/balance", handler.LedgerGetBalance)
		r.Get("/", handler.LedgerGetTransactionsByAccountID)
	})
}
