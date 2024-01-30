package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/albertwidi/ftest/ledger"
	"github.com/shopspring/decimal"
)

// Handler provides http handlers for all ledger endpoints.
type Handler struct {
	ld *ledger.Ledger
}

func New(ld *ledger.Ledger) *Handler {
	return &Handler{ld: ld}
}

type TransferRequest struct {
	FromAccount string `json:"from_account"`
	ToAccount   string `json:"to_account"`
	Amount      string `json:"amount"`
}

type TransferResponse struct {
	TransactionID string `json:"transaction_id"`
}

func (h *Handler) LedgerTransfer(w http.ResponseWriter, r *http.Request) {
	out, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error(err.Error())
		writeError(w, ErrorResponse{
			Message: "failed to read json body request",
			code:    http.StatusBadRequest,
		})
		return
	}

	req := TransferRequest{}
	if err := json.Unmarshal(out, &req); err != nil {
		slog.Error(err.Error())
		writeError(w, ErrorResponse{
			Message: "invalid transfer request format",
			code:    http.StatusBadRequest,
		})
		return
	}

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		slog.Error(err.Error())
		writeError(w, ErrorResponse{
			Message: "invalid amount for transfer",
			code:    http.StatusBadRequest,
		})
		return
	}

	txID, err := h.ld.Transfer(r.Context(), ledger.Transfer{
		FromAccount: req.FromAccount,
		ToAccount:   req.ToAccount,
		Amount:      amount,
	})
	if err != nil {
		slog.Error(err.Error())
		writeError(w, ErrorResponse{
			Message: "failed to transfer",
			code:    http.StatusInternalServerError,
		})
		return
	}

	out, err = json.Marshal(TransferResponse{TransactionID: txID})
	if err != nil {
		slog.Error(err.Error())
		writeError(w, ErrorResponse{
			Message: "failed to marshal response to client",
			code:    http.StatusInternalServerError,
		})
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(out)
}

type GetBalanceResponse struct {
	AccountID   string `json:"account_id"`
	Balance     string `json:"available_balance"`
	LastUpdated string `json:"last_updated"`
}

func (h *Handler) LedgerGetBalance(w http.ResponseWriter, r *http.Request) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		slog.Error(err.Error())
		writeError(w, ErrorResponse{
			Message: "invalid parameter for get balance query",
			code:    http.StatusBadRequest,
		})
		return
	}
	accountID := query.Get("account_id")
	if accountID == "" {
		writeError(w, ErrorResponse{
			Message: "account_id cannot be empty",
			code:    http.StatusBadRequest,
		})
		return
	}
	balance, err := h.ld.GetAccountBalance(r.Context(), accountID)
	if err != nil {
	}

	resp := GetBalanceResponse{
		AccountID:   balance.AccountID,
		Balance:     balance.Balance.String(),
		LastUpdated: balance.UpdatedAt.String(),
	}
	out, err := json.Marshal(resp)
	if err != nil {
		slog.Error(err.Error())
		writeError(w, ErrorResponse{
			Message: "failed to marshal response to client",
			code:    http.StatusInternalServerError,
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(out)
}

// GetTransactionsResponse is essentially list of transactions from the ledger. Because as of now
// we only have transfer which always 1:1 from user to user, we can use it for now.
type GetTransactionsResponse struct {
	Transactions []LedgerEntryResponse `json:"transactions"`
}

type LedgerEntryResponse struct {
	TransactionID string `json:"transaction_id"`
	AccountID     string `json:"account_id"`
	Amount        string `json:"amount"`
	CreatedAt     string `json:"created_at"`
}

func (h *Handler) LedgerGetTransactionsByAccountID(w http.ResponseWriter, r *http.Request) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		slog.Error(err.Error())
		writeError(w, ErrorResponse{
			Message: "invalid parameter for get balance query",
			code:    http.StatusBadRequest,
		})
		return
	}
	accountID := query.Get("account_id")
	if accountID == "" {
		writeError(w, ErrorResponse{
			Message: "account_id cannot be empty",
			code:    http.StatusBadRequest,
		})
		return
	}

	entries, err := h.ld.GetAccountLedgerEntries(r.Context(), accountID)
	if err != nil {
	}

	resp := GetTransactionsResponse{
		Transactions: make([]LedgerEntryResponse, len(entries)),
	}
	for idx, entry := range entries {
		resp.Transactions[idx] = LedgerEntryResponse{
			TransactionID: entry.TransactionID,
			AccountID:     entry.AccountID,
			Amount:        entry.Amount.String(),
			CreatedAt:     entry.CreatedAt.String(),
		}
	}

	out, err := json.Marshal(resp)
	if err != nil {
		slog.Error(err.Error())
		writeError(w, ErrorResponse{
			Message: "failed to marshal response to client",
			code:    http.StatusInternalServerError,
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(out)
}

type ErrorResponse struct {
	Message string `json:"message"`
	code    int
}

func writeError(w http.ResponseWriter, response ErrorResponse) {
	out, err := json.Marshal(response)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(response.code)
	w.Write(out)
}
