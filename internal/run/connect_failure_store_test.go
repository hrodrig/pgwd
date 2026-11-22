package run

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hrodrig/pgwd/internal/store"
)

type recordingStore struct {
	memStore
	inserted []store.Record
}

func (r *recordingStore) Insert(_ context.Context, rec store.Record) error {
	r.inserted = append(r.inserted, rec)
	r.states = append([]string{rec.State}, r.states...)
	return nil
}

func TestPersistConnectFailure_nilStore(t *testing.T) {
	PersistConnectFailure(context.Background(), nil, "cl", "c", "ns", "db", errors.New("down"))
}

func TestPersistConnectFailure_insertsConnectFailure(t *testing.T) {
	st := &recordingStore{}
	PersistConnectFailure(context.Background(), st, "cl", "c", "ns", "db", errors.New("connection refused"))
	if len(st.inserted) != 1 {
		t.Fatalf("inserted = %d, want 1", len(st.inserted))
	}
	rec := st.inserted[0]
	if rec.State != "connect_failure" || rec.Threshold != "connect_failure" {
		t.Fatalf("rec = %+v", rec)
	}
	if rec.Client != "c" || rec.Cluster != "cl" || rec.Namespace != "ns" || rec.Database != "db" {
		t.Fatalf("labels = %+v", rec)
	}
}

func TestPersistConnectFailure_tooManyClients(t *testing.T) {
	st := &recordingStore{}
	err533 := &pgconn.PgError{Code: "53300", Message: "too many clients"}
	PersistConnectFailure(context.Background(), st, "cl", "c", "", "db", err533)
	if st.inserted[0].Threshold != "too_many_clients" {
		t.Fatalf("threshold = %q", st.inserted[0].Threshold)
	}
	if st.inserted[0].State != "connect_failure" {
		t.Fatalf("state = %q", st.inserted[0].State)
	}
}
