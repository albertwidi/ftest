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
	"time"

	_ "github.com/lib/pq"

	"github.com/albertwidi/ftest/handler"
	"github.com/albertwidi/ftest/ledger"
	"github.com/go-chi/chi/v5"
)

type config struct {
	postgresDSN string
	servicePort string
	// delayStart is used as a quick hack to pause before starting the service. This is useful when we want
	// to wait for other service in docker-compose.
	delayStart string
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
	delayStart := os.Getenv("DELAY_START")
	if delayStart == "" {
		delayStart = "0s"
	}
	return config{
		postgresDSN: dsn,
		servicePort: servicePort,
		delayStart:  delayStart,
	}
}

func main() {
	config := loadConfig()

	dur, err := time.ParseDuration(config.delayStart)
	if err != nil {
		panic(err)
	}
	time.Sleep(dur)

	ctxSignal, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer cancel()

	db, err := sql.Open("postgres", config.postgresDSN)
	if err != nil {
		panic(err)
	}

	ld := ledger.New(db)
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

func handle(ld *ledger.Ledger, r chi.Router) {
	handler := handler.New(ld)
	r.Route("/v1/ledger", func(r chi.Router) {
		r.Post("/transfer", handler.LedgerTransfer)
		r.Post("/account", handler.LedgerCreateAccount)
		r.Get("/balance", handler.LedgerGetBalance)
		r.Get("/", handler.LedgerGetTransactionsByAccountID)
	})
}
