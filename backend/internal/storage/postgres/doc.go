// Package postgres persists owner-scoped chat state, research leases, immutable
// evidence snapshots and reusable retrieval caches.
//
// It deliberately retains parameterized pgx queries instead of introducing an ORM
// during the structural refactor. Admission locks and quota increments remain in
// one transaction, and cached documents retain the existing schema and timestamps.
package postgres
