package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"table-for-you/backend/internal/domain"
)

// Finish atomically stores the terminal result, immutable evidence and assistant message.
// A run already finished by another lifecycle operation is left unchanged.
func (s *Store) Finish(ctx context.Context, r domain.Run, status, message string, result *domain.Result, use domain.Usage) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return persistenceError("finish", err)
	}
	defer rollbackTransaction(tx)
	var b []byte
	if result != nil {
		b, err = json.Marshal(result)
		if err != nil {
			return persistenceError("finish", err)
		}
	}
	u, err := json.Marshal(use)
	if err != nil {
		return persistenceError("finish", err)
	}
	tag, err := tx.Exec(ctx, "UPDATE runs SET status=$2,result=$3,usage=$4,error=$5,finished_at=now() WHERE id=$1 AND status='running'", r.ID, status, b, u, message)
	if err != nil {
		return persistenceError("finish", err)
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	if result != nil {
		for _, d := range result.Sources {
			snap, err := json.Marshal(d)
			if err != nil {
				return persistenceError("finish", err)
			}
			if _, err = tx.Exec(ctx, "INSERT INTO run_evidence(run_id,source_id,snapshot) VALUES($1,$2,$3) ON CONFLICT DO NOTHING", r.ID, d.ID, snap); err != nil {
				return persistenceError("finish", err)
			}
		}
	}
	summary := message
	if result != nil && result.Clarification != "" {
		summary = result.Clarification
	} else if result != nil && result.Answer != "" {
		summary = result.Answer
	} else if result != nil {
		summary = fmt.Sprintf("Research finished: %d confirmed restaurants; %d options need confirmation.", len(result.Confirmed), len(result.Alternatives))
		if len(result.Limitations) > 0 {
			summary += " " + result.Limitations[0]
		}
	}
	if _, err = tx.Exec(ctx, "INSERT INTO messages(id,conversation_id,role,text,sequence) SELECT $1,$2,'assistant',$3,coalesce(max(sequence),0)+1 FROM messages WHERE conversation_id=$2", uuid.NewString(), r.ConversationID, summary); err != nil {
		return persistenceError("finish", err)
	}
	if _, err = tx.Exec(ctx, "UPDATE conversations SET updated_at=now() WHERE id=$1", r.ConversationID); err != nil {
		return persistenceError("finish", err)
	}
	if _, err = tx.Exec(ctx, "INSERT INTO run_events(run_id,sequence,type,message) SELECT $1,coalesce(max(sequence),0)+1,$2,$3 FROM run_events WHERE run_id=$1", r.ID, "run."+status, summary); err != nil {
		return persistenceError("finish", err)
	}
	return tx.Commit(ctx)
}
