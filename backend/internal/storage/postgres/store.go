package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"nebulaiq/internal/domain"
	"nebulaiq/migrations"
)

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("conversation changed or already has active research")
var ErrCapacity = errors.New("research capacity or daily quota reached")

type Store struct{ Pool *pgxpool.Pool }

func Open(ctx context.Context, url string) (*Store, error) {
	c, e := pgxpool.ParseConfig(url)
	if e != nil {
		return nil, errors.New("invalid DATABASE_URL")
	}
	c.MaxConns = 5
	c.ConnConfig.ConnectTimeout = 5 * time.Second
	p, e := pgxpool.NewWithConfig(ctx, c)
	if e != nil {
		return nil, e
	}
	if e = p.Ping(ctx); e != nil {
		p.Close()
		return nil, errors.New("PostgreSQL connection failed")
	}
	return &Store{p}, nil
}
func (s *Store) Close() { s.Pool.Close() }
func (s *Store) Migrate(ctx context.Context) error {
	c, e := s.Pool.Acquire(ctx)
	if e != nil {
		return e
	}
	defer c.Release()
	if _, e = c.Exec(ctx, "SELECT pg_advisory_lock(918372)"); e != nil {
		return e
	}
	defer c.Exec(context.Background(), "SELECT pg_advisory_unlock(918372)")
	if _, e = c.Exec(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())"); e != nil {
		return e
	}
	files, e := migrations.Files.ReadDir(".")
	if e != nil {
		return e
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		var done bool
		if e = c.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", f.Name()).Scan(&done); e != nil {
			return e
		}
		if done {
			continue
		}
		b, e := migrations.Files.ReadFile(f.Name())
		if e != nil {
			return e
		}
		tx, e := c.Begin(ctx)
		if e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, string(b)); e == nil {
			_, e = tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES($1)", f.Name())
		}
		if e != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("migration %s failed: %w", f.Name(), e)
		}
		if e = tx.Commit(ctx); e != nil {
			return e
		}
	}
	return nil
}
func Hash(v string) string { h := sha256.Sum256([]byte(v)); return hex.EncodeToString(h[:]) }
func (s *Store) NewSession(ctx context.Context) (token string, err error) {
	token = uuid.NewString() + uuid.NewString()
	owner := uuid.NewString()
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return "", e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "INSERT INTO owners(id) VALUES($1)", owner); e != nil {
		return "", e
	}
	if _, e = tx.Exec(ctx, "INSERT INTO browser_sessions(id,owner_id,token_hash,expires_at) VALUES($1,$2,$3,now()+interval '7 days')", uuid.NewString(), owner, Hash(token)); e != nil {
		return "", e
	}
	return token, tx.Commit(ctx)
}
func (s *Store) Owner(ctx context.Context, token string) (string, error) {
	var owner string
	e := s.Pool.QueryRow(ctx, "SELECT owner_id::text FROM browser_sessions WHERE token_hash=$1 AND expires_at>now() AND revoked_at IS NULL", Hash(token)).Scan(&owner)
	if errors.Is(e, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return owner, e
}
func (s *Store) Revoke(ctx context.Context, token string) error {
	_, e := s.Pool.Exec(ctx, "UPDATE browser_sessions SET revoked_at=now() WHERE token_hash=$1", Hash(token))
	return e
}
func (s *Store) CreateConversation(ctx context.Context, owner string) (domain.Conversation, error) {
	v := domain.Conversation{ID: uuid.NewString(), Title: "New dining research", Requirements: domain.Requirements{}, UpdatedAt: time.Now().UTC()}
	_, e := s.Pool.Exec(ctx, "INSERT INTO conversations(id,owner_id) VALUES($1,$2)", v.ID, owner)
	return v, e
}
func (s *Store) List(ctx context.Context, owner string) ([]domain.Conversation, error) {
	rows, e := s.Pool.Query(ctx, "SELECT id::text,title,requirements,version,updated_at FROM conversations WHERE owner_id=$1 AND updated_at>now()-interval '7 days' ORDER BY updated_at DESC LIMIT 50", owner)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.Conversation{}
	for rows.Next() {
		var v domain.Conversation
		var b []byte
		if e = rows.Scan(&v.ID, &v.Title, &b, &v.Version, &v.UpdatedAt); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(b, &v.Requirements); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) Conversation(ctx context.Context, owner, id string) (domain.Conversation, error) {
	var v domain.Conversation
	var b []byte
	e := s.Pool.QueryRow(ctx, "SELECT id::text,title,requirements,version,updated_at FROM conversations WHERE id=$1 AND owner_id=$2 AND updated_at>now()-interval '7 days'", id, owner).Scan(&v.ID, &v.Title, &b, &v.Version, &v.UpdatedAt)
	if errors.Is(e, pgx.ErrNoRows) {
		return v, ErrNotFound
	}
	if e != nil {
		return v, e
	}
	if e = json.Unmarshal(b, &v.Requirements); e != nil {
		return v, e
	}
	rows, e := s.Pool.Query(ctx, "SELECT id::text,role,text,created_at FROM messages WHERE conversation_id=$1 ORDER BY sequence", id)
	if e != nil {
		return v, e
	}
	for rows.Next() {
		var m domain.Message
		if e = rows.Scan(&m.ID, &m.Role, &m.Text, &m.CreatedAt); e != nil {
			rows.Close()
			return v, e
		}
		v.Messages = append(v.Messages, m)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return v, e
	}
	rows, e = s.Pool.Query(ctx, "SELECT id::text,conversation_id::text,status,requirements,result,error,created_at,usage FROM runs WHERE conversation_id=$1 ORDER BY created_at DESC LIMIT 10", id)
	if e != nil {
		return v, e
	}
	defer rows.Close()
	for rows.Next() {
		r, e := scanRun(rows)
		if e != nil {
			return v, e
		}
		v.Runs = append(v.Runs, r)
	}
	return v, rows.Err()
}
func scanRun(row pgx.Row) (domain.Run, error) {
	var r domain.Run
	var req, res, use []byte
	e := row.Scan(&r.ID, &r.ConversationID, &r.Status, &req, &res, &r.Error, &r.CreatedAt, &use)
	if errors.Is(e, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	if e != nil {
		return r, e
	}
	if e = json.Unmarshal(req, &r.Requirements); e != nil {
		return r, e
	}
	if len(res) > 0 {
		if e = json.Unmarshal(res, &r.Result); e != nil {
			return r, e
		}
	}
	e = json.Unmarshal(use, &r.Usage)
	return r, e
}
func (s *Store) Run(ctx context.Context, owner, id string) (domain.Run, error) {
	return scanRun(s.Pool.QueryRow(ctx, "SELECT r.id::text,r.conversation_id::text,r.status,r.requirements,r.result,r.error,r.created_at,r.usage FROM runs r JOIN conversations c ON c.id=r.conversation_id WHERE r.id=$1 AND c.owner_id=$2 AND c.updated_at>now()-interval '7 days'", id, owner))
}

type ModeLimits struct {
	Mode                    string
	OwnerQuota, GlobalQuota int
}

var ErrGroundedCapacity = errors.New("Google-grounded daily quota reached")

func (s *Store) Accept(ctx context.Context, owner, conversation, request, text string, version int, refresh bool, timeout time.Duration, ownerQuota, globalQuota int, modes ...ModeLimits) (domain.Run, bool, error) {
	var r domain.Run
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return r, false, e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(918373)"); e != nil {
		return r, false, e
	}
	var current int
	var req []byte
	e = tx.QueryRow(ctx, "SELECT version,requirements FROM conversations WHERE id=$1 AND owner_id=$2 AND updated_at>now()-interval '7 days' FOR UPDATE", conversation, owner).Scan(&current, &req)
	if errors.Is(e, pgx.ErrNoRows) {
		return r, false, ErrNotFound
	}
	if e != nil {
		return r, false, e
	}
	old, e := scanRun(tx.QueryRow(ctx, "SELECT id::text,conversation_id::text,status,requirements,result,error,created_at,usage FROM runs WHERE conversation_id=$1 AND request_id=$2", conversation, request))
	if e == nil {
		return old, false, nil
	}
	if !errors.Is(e, ErrNotFound) {
		return r, false, e
	}
	if version != current {
		return r, false, ErrConflict
	}
	if _, e = tx.Exec(ctx, "UPDATE runs SET status='interrupted',error='Service interrupted; retry explicitly.',finished_at=now() WHERE status='running' AND lease_until<now()"); e != nil {
		return r, false, e
	}
	var active, total, owned, global int
	if e = tx.QueryRow(ctx, "SELECT count(*) FILTER(WHERE conversation_id=$1),count(*) FROM runs WHERE status='running'", conversation).Scan(&active, &total); e != nil {
		return r, false, e
	}
	if e = tx.QueryRow(ctx, "SELECT coalesce((SELECT accepted FROM owner_usage_days WHERE owner_id=$1 AND day=(now() AT TIME ZONE 'UTC')::date),0),coalesce((SELECT accepted FROM usage_days WHERE day=(now() AT TIME ZONE 'UTC')::date),0)", owner).Scan(&owned, &global); e != nil {
		return r, false, e
	}
	if active > 0 {
		return r, false, ErrConflict
	}
	if total >= 2 || owned >= ownerQuota || global >= globalQuota {
		return r, false, ErrCapacity
	}
	mode := ModeLimits{Mode: "standard"}
	if len(modes) > 0 {
		mode = modes[0]
	}
	if mode.Mode == "google_grounded" {
		var groundedOwned, groundedGlobal int
		if e = tx.QueryRow(ctx, "SELECT coalesce((SELECT accepted FROM grounded_owner_usage_days WHERE owner_id=$1 AND day=(now() AT TIME ZONE 'UTC')::date),0),coalesce((SELECT accepted FROM grounded_usage_days WHERE day=(now() AT TIME ZONE 'UTC')::date),0)", owner).Scan(&groundedOwned, &groundedGlobal); e != nil {
			return r, false, e
		}
		if groundedOwned >= mode.OwnerQuota || groundedGlobal >= mode.GlobalQuota {
			return r, false, ErrGroundedCapacity
		}
		if _, e = tx.Exec(ctx, "INSERT INTO grounded_usage_days(day,accepted) VALUES((now() AT TIME ZONE 'UTC')::date,1) ON CONFLICT(day) DO UPDATE SET accepted=grounded_usage_days.accepted+1"); e != nil {
			return r, false, e
		}
		if _, e = tx.Exec(ctx, "INSERT INTO grounded_owner_usage_days(owner_id,day,accepted) VALUES($1,(now() AT TIME ZONE 'UTC')::date,1) ON CONFLICT(owner_id,day) DO UPDATE SET accepted=grounded_owner_usage_days.accepted+1", owner); e != nil {
			return r, false, e
		}
	}
	r = domain.Run{ID: uuid.NewString(), ConversationID: conversation, Status: "running", CreatedAt: time.Now().UTC(), Usage: domain.Usage{Mode: mode.Mode}}
	if e = json.Unmarshal(req, &r.Requirements); e != nil {
		return r, false, e
	}
	if _, e = tx.Exec(ctx, "INSERT INTO runs(id,conversation_id,request_id,requirements,status,refresh,deadline,lease_until,usage) VALUES($1,$2,$3,$4,'running',$5,$6,now()+interval '30 seconds',$7)", r.ID, conversation, request, req, refresh, time.Now().Add(timeout), mustJSON(r.Usage)); e != nil {
		return r, false, e
	}
	if _, e = tx.Exec(ctx, "INSERT INTO messages(id,conversation_id,role,text,sequence) SELECT $1,$2,'user',$3,coalesce(max(sequence),0)+1 FROM messages WHERE conversation_id=$2", uuid.NewString(), conversation, text); e != nil {
		return r, false, e
	}
	title := text
	if len([]rune(title)) > 65 {
		title = string([]rune(title)[:65]) + "…"
	}
	if _, e = tx.Exec(ctx, "UPDATE conversations SET version=version+1,updated_at=now(),title=CASE WHEN version=0 THEN $2 ELSE title END WHERE id=$1", conversation, title); e != nil {
		return r, false, e
	}
	if _, e = tx.Exec(ctx, "INSERT INTO usage_days(day,accepted) VALUES((now() AT TIME ZONE 'UTC')::date,1) ON CONFLICT(day) DO UPDATE SET accepted=usage_days.accepted+1"); e != nil {
		return r, false, e
	}
	if _, e = tx.Exec(ctx, "INSERT INTO owner_usage_days(owner_id,day,accepted) VALUES($1,(now() AT TIME ZONE 'UTC')::date,1) ON CONFLICT(owner_id,day) DO UPDATE SET accepted=owner_usage_days.accepted+1", owner); e != nil {
		return r, false, e
	}
	return r, true, tx.Commit(ctx)
}
func (s *Store) Requirements(ctx context.Context, r domain.Run, req domain.Requirements) error {
	b, e := json.Marshal(req)
	if e != nil {
		return e
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	tag, e := tx.Exec(ctx, "UPDATE runs SET requirements=$2 WHERE id=$1 AND status='running'", r.ID, b)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	if _, e = tx.Exec(ctx, "UPDATE conversations SET requirements=$2 WHERE id=$1", r.ConversationID, b); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (s *Store) Event(ctx context.Context, run, kind, message string) error {
	_, e := s.Pool.Exec(ctx, "INSERT INTO run_events(run_id,sequence,type,message) SELECT $1,coalesce(max(sequence),0)+1,$2,$3 FROM run_events WHERE run_id=$1", run, kind, message)
	return e
}
func (s *Store) Events(ctx context.Context, run string, after int64) ([]domain.Event, error) {
	rows, e := s.Pool.Query(ctx, "SELECT sequence,type,message,created_at FROM run_events WHERE run_id=$1 AND sequence>$2 ORDER BY sequence LIMIT 200", run, after)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.Event{}
	for rows.Next() {
		var v domain.Event
		if e = rows.Scan(&v.Sequence, &v.Type, &v.Message, &v.CreatedAt); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) Heartbeat(ctx context.Context, run string) (bool, error) {
	var cancel bool
	e := s.Pool.QueryRow(ctx, "UPDATE runs SET lease_until=now()+interval '30 seconds' WHERE id=$1 AND status='running' RETURNING cancel_requested", run).Scan(&cancel)
	return cancel, e
}
func (s *Store) Finish(ctx context.Context, r domain.Run, status, message string, result *domain.Result, use domain.Usage) error {
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var b []byte
	if result != nil {
		b, e = json.Marshal(result)
		if e != nil {
			return e
		}
	}
	u, e := json.Marshal(use)
	if e != nil {
		return e
	}
	tag, e := tx.Exec(ctx, "UPDATE runs SET status=$2,result=$3,usage=$4,error=$5,finished_at=now() WHERE id=$1 AND status='running'", r.ID, status, b, u, message)
	if e != nil {
		return e
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	if result != nil {
		for _, d := range result.Sources {
			snap, e := json.Marshal(d)
			if e != nil {
				return e
			}
			if _, e = tx.Exec(ctx, "INSERT INTO run_evidence(run_id,source_id,snapshot) VALUES($1,$2,$3) ON CONFLICT DO NOTHING", r.ID, d.ID, snap); e != nil {
				return e
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
	if _, e = tx.Exec(ctx, "INSERT INTO messages(id,conversation_id,role,text,sequence) SELECT $1,$2,'assistant',$3,coalesce(max(sequence),0)+1 FROM messages WHERE conversation_id=$2", uuid.NewString(), r.ConversationID, summary); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "UPDATE conversations SET updated_at=now() WHERE id=$1", r.ConversationID); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "INSERT INTO run_events(run_id,sequence,type,message) SELECT $1,coalesce(max(sequence),0)+1,$2,$3 FROM run_events WHERE run_id=$1", r.ID, "run."+status, summary); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (s *Store) Cancel(ctx context.Context, owner, run string) error {
	tag, e := s.Pool.Exec(ctx, "UPDATE runs SET cancel_requested=true WHERE id=$1 AND status='running' AND conversation_id IN(SELECT id FROM conversations WHERE owner_id=$2)", run, owner)
	if e != nil {
		return e
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *Store) Delete(ctx context.Context, owner, id string) error {
	tag, e := s.Pool.Exec(ctx, "DELETE FROM conversations WHERE id=$1 AND owner_id=$2 AND NOT EXISTS(SELECT 1 FROM runs WHERE conversation_id=$1 AND status='running')", id, owner)
	if e != nil {
		return e
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	return nil
}
func (s *Store) Recover(ctx context.Context) error {
	_, e := s.Pool.Exec(ctx, "UPDATE runs SET status='interrupted',error='Service interrupted; retry explicitly.',finished_at=now() WHERE status='running' AND lease_until<now()")
	return e
}
func (s *Store) Cleanup(ctx context.Context) error {
	queries := []string{
		"DELETE FROM conversations WHERE id IN(SELECT id FROM conversations WHERE updated_at<now()-interval '7 days' AND NOT EXISTS(SELECT 1 FROM runs WHERE conversation_id=conversations.id AND status='running') LIMIT 100)",
		"DELETE FROM browser_sessions WHERE id IN(SELECT id FROM browser_sessions WHERE expires_at<now() OR revoked_at IS NOT NULL LIMIT 100)",
		"DELETE FROM search_cache WHERE (owner_id,request_hash) IN(SELECT owner_id,request_hash FROM search_cache WHERE expires_at<now() LIMIT 100)",
		"DELETE FROM source_documents WHERE id IN(SELECT id FROM source_documents WHERE last_used_at<now()-interval '7 days' OR (SELECT coalesce(sum(pg_column_size(payload)),0) FROM source_documents)>104857600 ORDER BY last_used_at LIMIT 50)",
		"DELETE FROM extractions WHERE request_hash IN(SELECT request_hash FROM extractions WHERE last_used_at<now()-interval '7 days' OR (SELECT coalesce(sum(pg_column_size(payload)),0) FROM extractions)>104857600 ORDER BY last_used_at LIMIT 50)",
	}
	for _, q := range queries {
		if _, e := s.Pool.Exec(ctx, q); e != nil {
			return e
		}
	}
	return nil
}

func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }
