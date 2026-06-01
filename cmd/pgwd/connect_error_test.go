package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestIsTooManyClientsError(t *testing.T) {
	t.Parallel()

	italian := &pgconn.PgError{
		Severity: "ERRORE",
		Code:     "53300",
		Message:  "spiacenti, troppi client già connessi",
	}
	english := &pgconn.PgError{
		Severity: "ERROR",
		Code:     "53300",
		Message:  "sorry, too many clients already",
	}
	other := &pgconn.PgError{
		Severity: "ERROR",
		Code:     "28P01",
		Message:  "password authentication failed",
	}
	wrapped := fmt.Errorf("connect: %w", italian)

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"generic", errors.New("connection refused"), false},
		{"english message 53300", english, true},
		{"italian message 53300", italian, true},
		{"wrapped connect error", wrapped, true},
		{"other sqlstate", other, false},
		{"english text only no pg error", errors.New("sorry, too many clients already"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isTooManyClientsError(tt.err); got != tt.want {
				t.Fatalf("isTooManyClientsError() = %v, want %v (err=%v)", got, tt.want, tt.err)
			}
		})
	}
}
