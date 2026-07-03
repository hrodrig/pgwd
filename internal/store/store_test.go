package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestStore_InsertAndEvict(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "pgwd.db")

	st, err := Open(path, 5)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	rec := Record{
		Client: "test", Cluster: "c1", Database: "db1",
		Total: 10, Active: 2, Idle: 8, Stale: 0, MaxConnections: 100,
		State: "ok", Threshold: "",
	}

	for i := 0; i < 10; i++ {
		if err := st.Insert(ctx, rec); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	var count int
	err = st.db.QueryRowContext(ctx, "SELECT count(*) FROM metrics").Scan(&count)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5 rows (FIFO eviction), got %d", count)
	}
}

func TestStore_LastStates(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "pgwd.db")

	st, err := Open(path, 1000)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	rec := Record{Client: "c", Cluster: "cl", Database: "d", MaxConnections: 100}
	for _, state := range []string{"attention", "alert", "ok"} {
		rec.State = state
		rec.Threshold = state
		if err := st.Insert(ctx, rec); err != nil {
			t.Fatalf("Insert: %v", err)
		}
		time.Sleep(2 * time.Millisecond) // ensure distinct ts for ordering
	}

	states, err := st.LastStates(ctx, "c", "cl", "d", 3)
	if err != nil {
		t.Fatalf("LastStates: %v", err)
	}
	if len(states) != 3 {
		t.Errorf("LastStates: want 3, got %d", len(states))
	}
	if states[0] != "ok" || states[1] != "alert" || states[2] != "attention" {
		t.Errorf("LastStates: got %v (newest first)", states)
	}
}

func TestStore_LatestRecords(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "pgwd.db")

	st, err := Open(path, 1000)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	// Insert from two targets
	for _, r := range []Record{
		{Client: "c1", Cluster: "cl1", Database: "d1", Total: 10, Active: 2, Idle: 8, Stale: 0, MaxConnections: 100, State: "ok"},
		{Client: "c1", Cluster: "cl1", Database: "d1", Total: 15, Active: 5, Idle: 10, Stale: 0, MaxConnections: 100, State: "attention"},
		{Client: "c2", Cluster: "cl2", Database: "d2", Total: 20, Active: 8, Idle: 12, Stale: 1, MaxConnections: 200, State: "ok"},
	} {
		if err := st.Insert(ctx, r); err != nil {
			t.Fatalf("Insert: %v", err)
		}
		time.Sleep(2 * time.Millisecond)
	}

	recs, err := st.LatestRecords(ctx)
	if err != nil {
		t.Fatalf("LatestRecords: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("LatestRecords: want 2 targets, got %d", len(recs))
	}
	recByTarget := make(map[string]Record)
	for _, r := range recs {
		recByTarget[r.Client+"/"+r.Database] = r
	}
	if r, ok := recByTarget["c1/d1"]; !ok {
		t.Error("LatestRecords: missing c1/d1")
	} else if r.Total != 15 || r.State != "attention" {
		t.Errorf("c1/d1: got Total=%d State=%q, want 15/attention", r.Total, r.State)
	}
	if r, ok := recByTarget["c2/d2"]; !ok {
		t.Error("LatestRecords: missing c2/d2")
	} else if r.Total != 20 || r.Stale != 1 {
		t.Errorf("c2/d2: got Total=%d Stale=%d, want 20/1", r.Total, r.Stale)
	}
}

func TestStore_LastStates_zeroN(t *testing.T) {
	ctx := context.Background()
	st, err := Open(filepath.Join(t.TempDir(), "z.db"), 10)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	states, err := st.LastStates(ctx, "c", "cl", "d", 0)
	if err != nil || states != nil {
		t.Fatalf("states=%v err=%v", states, err)
	}
}

func TestStore_Open_defaultMaxMetrics(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "d.db"), 0)
	if err != nil {
		t.Fatal(err)
	}
	st.Close()
}

func TestQueryAllMetricsReadOnly_Empty(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.db")
	st, err := Open(path, 100)
	if err != nil {
		t.Fatal(err)
	}
	st.Close()
	rows, err := QueryAllMetricsReadOnly(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("want 0 rows, got %d", len(rows))
	}
}
