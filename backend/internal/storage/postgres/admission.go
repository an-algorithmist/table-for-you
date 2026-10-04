package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"table-for-you/backend/internal/domain"
)

// ModeLimits applies an additional daily allowance to an optional research mode.
type ModeLimits struct {
	Mode                    string
	OwnerQuota, GlobalQuota int
}

// Accept atomically admits a turn, enforcing ownership, idempotency, version and quotas.
// The boolean reports whether a new run was created; a duplicate request returns the existing run.
func (s *Store) Accept(ctx context.Context, owner, conversation, request, text string, version int, refresh bool, timeout time.Duration, ownerQuota, globalQuota int, modes ...ModeLimits) (domain.Run, bool, error) {
	var r domain.Run
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return r, false, persistenceError("accept", err)
	}
	defer rollbackTransaction(tx)
	// Serialize quota admission across owners; per-conversation row locks alone
	// would allow concurrent requests to oversubscribe the global allowance.
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(918373)"); err != nil {
		return r, false, persistenceError("accept", err)
	}
	var current int
	var req []byte
	err = tx.QueryRow(ctx, "SELECT version,requirements FROM conversations WHERE id=$1 AND owner_id=$2 AND updated_at>now()-interval '7 days' FOR UPDATE", conversation, owner).Scan(&current, &req)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, false, ErrNotFound
	}
	if err != nil {
		return r, false, persistenceError("accept", err)
	}
	old, err := scanRun(tx.QueryRow(ctx, "SELECT id::text,conversation_id::text,status,requirements,result,error,created_at,usage FROM runs WHERE conversation_id=$1 AND request_id=$2", conversation, request))
	if err == nil {
		return old, false, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return r, false, persistenceError("accept", err)
	}
	if version != current {
		return r, false, ErrConflict
	}
	if _, err = tx.Exec(ctx, "UPDATE runs SET status='interrupted',error='Service interrupted; retry explicitly.',finished_at=now() WHERE status='running' AND lease_until<now()"); err != nil {
		return r, false, persistenceError("accept", err)
	}
	if err = checkAdmissionCapacity(ctx, tx, owner, conversation, ownerQuota, globalQuota); err != nil {
		return r, false, persistenceError("accept", err)
	}
	mode := ModeLimits{Mode: "standard"}
	if len(modes) > 0 {
		mode = modes[0]
	}
	if err = chargeGroundedQuota(ctx, tx, owner, mode); err != nil {
		return r, false, persistenceError("accept", err)
	}
	r = domain.Run{ID: uuid.NewString(), ConversationID: conversation, Status: "running", CreatedAt: time.Now().UTC(), Usage: domain.Usage{Mode: mode.Mode}}
	if err = json.Unmarshal(req, &r.Requirements); err != nil {
		return r, false, persistenceError("accept", err)
	}
	usageJSON, err := json.Marshal(r.Usage)
	if err != nil {
		return r, false, persistenceError("accept", err)
	}
	if _, err = tx.Exec(ctx, "INSERT INTO runs(id,conversation_id,request_id,requirements,status,refresh,deadline,lease_until,usage) VALUES($1,$2,$3,$4,'running',$5,$6,now()+interval '30 seconds',$7)", r.ID, conversation, request, req, refresh, time.Now().Add(timeout), usageJSON); err != nil {
		return r, false, persistenceError("accept", err)
	}
	if _, err = tx.Exec(ctx, "INSERT INTO messages(id,conversation_id,role,text,sequence) SELECT $1,$2,'user',$3,coalesce(max(sequence),0)+1 FROM messages WHERE conversation_id=$2", uuid.NewString(), conversation, text); err != nil {
		return r, false, persistenceError("accept", err)
	}
	title := text
	if len([]rune(title)) > 65 {
		title = string([]rune(title)[:65]) + "…"
	}
	if _, err = tx.Exec(ctx, "UPDATE conversations SET version=version+1,updated_at=now(),title=CASE WHEN version=0 THEN $2 ELSE title END WHERE id=$1", conversation, title); err != nil {
		return r, false, persistenceError("accept", err)
	}
	if err = chargeStandardQuota(ctx, tx, owner); err != nil {
		return r, false, persistenceError("accept", err)
	}
	return r, true, tx.Commit(ctx)
}

// checkAdmissionCapacity must run under the admission transaction's advisory lock.
// Expired leases have already been recovered, so only live work occupies capacity.
func checkAdmissionCapacity(ctx context.Context, tx pgx.Tx, owner, conversation string, ownerQuota, globalQuota int) error {
	var err error
	var active, total, owned, global int
	if err = tx.QueryRow(ctx, "SELECT count(*) FILTER(WHERE conversation_id=$1),count(*) FROM runs WHERE status='running'", conversation).Scan(&active, &total); err != nil {
		return persistenceError("check admission capacity", err)
	}
	if err = tx.QueryRow(ctx, "SELECT coalesce((SELECT accepted FROM owner_usage_days WHERE owner_id=$1 AND day=(now() AT TIME ZONE 'UTC')::date),0),coalesce((SELECT accepted FROM usage_days WHERE day=(now() AT TIME ZONE 'UTC')::date),0)", owner).Scan(&owned, &global); err != nil {
		return persistenceError("check admission capacity", err)
	}
	if active > 0 {
		return ErrConflict
	}
	if total >= 2 || owned >= ownerQuota || global >= globalQuota {
		return ErrCapacity
	}
	return nil
}

// The separate counters outlive deleted conversations and charge accepted turns,
// including clarification turns. Duplicate request IDs return before charging.
func chargeGroundedQuota(ctx context.Context, tx pgx.Tx, owner string, mode ModeLimits) error {
	var err error
	if mode.Mode == "google_grounded" {
		var groundedOwned, groundedGlobal int
		if err = tx.QueryRow(ctx, "SELECT coalesce((SELECT accepted FROM grounded_owner_usage_days WHERE owner_id=$1 AND day=(now() AT TIME ZONE 'UTC')::date),0),coalesce((SELECT accepted FROM grounded_usage_days WHERE day=(now() AT TIME ZONE 'UTC')::date),0)", owner).Scan(&groundedOwned, &groundedGlobal); err != nil {
			return persistenceError("charge grounded quota", err)
		}
		if groundedOwned >= mode.OwnerQuota || groundedGlobal >= mode.GlobalQuota {
			return ErrGroundedCapacity
		}
		if _, err = tx.Exec(ctx, "INSERT INTO grounded_usage_days(day,accepted) VALUES((now() AT TIME ZONE 'UTC')::date,1) ON CONFLICT(day) DO UPDATE SET accepted=grounded_usage_days.accepted+1"); err != nil {
			return persistenceError("charge grounded quota", err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO grounded_owner_usage_days(owner_id,day,accepted) VALUES($1,(now() AT TIME ZONE 'UTC')::date,1) ON CONFLICT(owner_id,day) DO UPDATE SET accepted=grounded_owner_usage_days.accepted+1", owner); err != nil {
			return persistenceError("charge grounded quota", err)
		}
	}
	return nil
}

func chargeStandardQuota(ctx context.Context, tx pgx.Tx, owner string) error {
	var err error
	if _, err = tx.Exec(ctx, "INSERT INTO usage_days(day,accepted) VALUES((now() AT TIME ZONE 'UTC')::date,1) ON CONFLICT(day) DO UPDATE SET accepted=usage_days.accepted+1"); err != nil {
		return persistenceError("charge standard quota", err)
	}
	if _, err = tx.Exec(ctx, "INSERT INTO owner_usage_days(owner_id,day,accepted) VALUES($1,(now() AT TIME ZONE 'UTC')::date,1) ON CONFLICT(owner_id,day) DO UPDATE SET accepted=owner_usage_days.accepted+1", owner); err != nil {
		return persistenceError("charge standard quota", err)
	}
	return nil
}
