package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestAlertCooldownSQLite_LongQueryRoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "m.db")
	st, err := Open(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	_, has, err := st.LastLongQueryAlert(ctx, "c", "cl", "db1")
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("unexpected prior cooldown")
	}
	now := time.Unix(1_700_000_000, 0)
	if err := st.SetLongQueryAlert(ctx, "c", "cl", "db1", now); err != nil {
		t.Fatal(err)
	}
	got, has, err := st.LastLongQueryAlert(ctx, "c", "cl", "db1")
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("want cooldown row")
	}
	if !got.Equal(now) {
		t.Fatalf("time: got %v want %v", got, now)
	}
}
