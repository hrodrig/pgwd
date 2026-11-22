package run

import (
	"context"
	"log"

	"github.com/hrodrig/pgwd/internal/store"
)

// PersistConnectFailure records an infrastructure outage in the metrics store so
// pgwd_state=connect_failure is visible on /metrics and resolution hysteresis can fire
// when connectivity returns. No-op when st is nil.
func PersistConnectFailure(ctx context.Context, st store.MetricsStorer, cluster, client, ns, db string, connectErr error) {
	if st == nil {
		return
	}
	thr := "connect_failure"
	if IsTooManyClientsError(connectErr) {
		thr = "too_many_clients"
	}
	rec := store.Record{
		Client:    client,
		Cluster:   cluster,
		Namespace: ns,
		Database:  db,
		State:     "connect_failure",
		Threshold: thr,
	}
	if err := st.Insert(ctx, rec); err != nil {
		log.Printf("store insert (connect failure): %v", err)
	}
}
