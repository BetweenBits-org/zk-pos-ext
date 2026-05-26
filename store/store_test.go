package store_test

import (
	"errors"
	"testing"

	"github.com/binance/zkmerkle-proof-of-solvency/zkpor/store"
	"github.com/go-sql-driver/mysql"
)

// TestConvertMySQLErr_TranslatesKnownNumbers locks the MySQL error
// number → sentinel mapping the engine relies on for retry decisions
// (timeout / interrupted) and graceful-degradation branches
// (table-not-found). Numbers mirror legacy
// src/utils.ConvertMysqlErrToDbErr.
func TestConvertMySQLErr_TranslatesKnownNumbers(t *testing.T) {
	cases := []struct {
		name   string
		in     error
		expect error
	}{
		{"interrupted (1317)", &mysql.MySQLError{Number: 1317}, store.ErrQueryInterrupted},
		{"timeout (3024)", &mysql.MySQLError{Number: 3024}, store.ErrQueryTimeout},
		{"table missing (1146)", &mysql.MySQLError{Number: 1146}, store.ErrTableNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := store.ConvertMySQLErr(tc.in)
			if !errors.Is(got, tc.expect) {
				t.Fatalf("ConvertMySQLErr(%v) = %v, want %v", tc.in, got, tc.expect)
			}
		})
	}
}

// TestConvertMySQLErr_PassesThroughOthers confirms unknown MySQL
// numbers and non-MySQL errors are returned unchanged.
func TestConvertMySQLErr_PassesThroughOthers(t *testing.T) {
	unknown := &mysql.MySQLError{Number: 9999, Message: "other"}
	if got := store.ConvertMySQLErr(unknown); got != unknown {
		t.Fatalf("unknown MySQL error not pass-through: got %v", got)
	}
	plain := errors.New("network down")
	if got := store.ConvertMySQLErr(plain); got != plain {
		t.Fatalf("non-MySQL error not pass-through: got %v", got)
	}
	if got := store.ConvertMySQLErr(nil); got != nil {
		t.Fatalf("nil should pass through nil, got %v", got)
	}
}

// TestIsNotFound exercises the small convenience wrapper.
func TestIsNotFound(t *testing.T) {
	if !store.IsNotFound(store.ErrNotFound) {
		t.Fatal("IsNotFound(ErrNotFound) = false")
	}
	if store.IsNotFound(errors.New("other")) {
		t.Fatal("IsNotFound on other error = true")
	}
	if store.IsNotFound(nil) {
		t.Fatal("IsNotFound(nil) = true")
	}
}
