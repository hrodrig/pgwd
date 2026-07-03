package postgres

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

type fakeRow struct {
	scan func(dest ...any) error
}

func (r fakeRow) Scan(dest ...any) error { return r.scan(dest...) }

type fakeQuerier struct {
	queryRow func(ctx context.Context, sql string, args ...any) pgx.Row
}

func (f *fakeQuerier) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return f.queryRow(ctx, sql, args...)
}

func TestLongQueryCount_zeroMinSeconds(t *testing.T) {
	n, err := LongQueryCount(context.Background(), &fakeQuerier{}, 0)
	if err != nil || n != 0 {
		t.Fatalf("got n=%d err=%v", n, err)
	}
}

func TestStats_mock(t *testing.T) {
	q := &fakeQuerier{
		queryRow: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return fakeRow{scan: func(dest ...any) error {
				*(dest[0].(*int)) = 2
				*(dest[1].(*int)) = 3
				*(dest[2].(*int)) = 5
				return nil
			}}
		},
	}
	s, err := Stats(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if s.Active != 2 || s.Idle != 3 || s.Total != 5 {
		t.Fatalf("stats = %+v", s)
	}
}

func TestMaxConnections_mock(t *testing.T) {
	q := &fakeQuerier{
		queryRow: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return fakeRow{scan: func(dest ...any) error {
				*(dest[0].(*int)) = 128
				return nil
			}}
		},
	}
	n, err := MaxConnections(context.Background(), q)
	if err != nil || n != 128 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func TestStaleCount_mock(t *testing.T) {
	q := &fakeQuerier{
		queryRow: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return fakeRow{scan: func(dest ...any) error {
				*(dest[0].(*int)) = 7
				return nil
			}}
		},
	}
	n, err := StaleCount(context.Background(), q, 300)
	if err != nil || n != 7 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func TestLongQueryCount_mock(t *testing.T) {
	q := &fakeQuerier{
		queryRow: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return fakeRow{scan: func(dest ...any) error {
				*(dest[0].(*int)) = 4
				return nil
			}}
		},
	}
	n, err := LongQueryCount(context.Background(), q, 30)
	if err != nil || n != 4 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}
