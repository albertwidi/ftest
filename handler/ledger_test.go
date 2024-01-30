package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/albertwidi/ftest/ledger"
)

// TestLedgerTransfer tests the transfer feature. This kind of test ensure we receive message in the format
// that we wanted(in JSON) with correct message and parameter.
func TestLedgerTransfer(t *testing.T) {
	ledger.BootstrapTest(t, testHandler.ld)
	t.Cleanup(func() {
		ledger.RestLedger(t, testHandler.ld)
	})

	req := TransferRequest{
		FromAccount: "b-fund",
		ToAccount:   "b-acc-1",
		Amount:      "10.1",
	}
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}

	httpReq := httptest.NewRequest("POST", "/", bytes.NewBuffer(out))
	w := httptest.NewRecorder()

	testHandler.LedgerTransfer(w, httpReq)
	if w.Code != http.StatusOK {
		t.Fatal("expect ok status from ledger transfer")
	}

	resp := TransferResponse{}
	httpResp := w.Result()

	out, err = io.ReadAll(httpResp.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer httpResp.Body.Close()

	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.TransactionID == "" {
		t.Fatal("got empty transaction id")
	}

	httpReq = httptest.NewRequest("GET", "/?account_id=b-acc-1", nil)
	w = httptest.NewRecorder()

	testHandler.LedgerGetBalance(w, httpReq)
	if w.Code != http.StatusOK {
		t.Fatal("expecting status ok from get balance")
	}

	balanceResp := GetBalanceResponse{}
	httpResp = w.Result()

	out, err = io.ReadAll(httpResp.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer httpResp.Body.Close()

	if err := json.Unmarshal(out, &balanceResp); err != nil {
		t.Fatal(err)
	}

	expect := GetBalanceResponse{
		AccountID: "b-acc-1",
		Balance:   "10.1",
	}
	if diff := cmp.Diff(expect, balanceResp, cmpopts.IgnoreFields(
		GetBalanceResponse{}, "LastUpdated",
	)); diff != "" {
		t.Fatalf("(-want/+got)\n BalanceResp:\n%s", diff)
	}

	// Get the transactions.
	httpReq = httptest.NewRequest("GET", "/?account_id=b-acc-1", nil)
	w = httptest.NewRecorder()

	testHandler.LedgerGetTransactionsByAccountID(w, httpReq)
	if w.Code != http.StatusOK {
		t.Fatal("expecting status ok from get balance")
	}

	transactionsResp := GetTransactionsResponse{}
	httpResp = w.Result()

	out, err = io.ReadAll(httpResp.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer httpResp.Body.Close()

	if err := json.Unmarshal(out, &transactionsResp); err != nil {
		t.Fatal(err)
	}

	expectTransactions := GetTransactionsResponse{
		Transactions: []LedgerEntryResponse{
			{
				AccountID: "b-acc-1",
				Amount:    "10.1",
			},
		},
	}
	if diff := cmp.Diff(expectTransactions, transactionsResp, cmpopts.IgnoreFields(
		LedgerEntryResponse{}, "TransactionID", "CreatedAt",
	)); diff != "" {
		t.Fatalf("(-want/+got) Transactions:\n%s", diff)
	}
}
