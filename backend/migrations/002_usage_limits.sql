-- Quotas survive conversation deletion; counters are independent of history rows.
CREATE TABLE usage_days (day date PRIMARY KEY, accepted integer NOT NULL DEFAULT 0);
CREATE TABLE owner_usage_days (owner_id uuid NOT NULL REFERENCES owners(id), day date NOT NULL, accepted integer NOT NULL DEFAULT 0, PRIMARY KEY(owner_id,day));
INSERT INTO usage_days(day,accepted) SELECT (created_at AT TIME ZONE 'UTC')::date,count(*) FROM runs GROUP BY 1;
INSERT INTO owner_usage_days(owner_id,day,accepted) SELECT c.owner_id,(r.created_at AT TIME ZONE 'UTC')::date,count(*) FROM runs r JOIN conversations c ON c.id=r.conversation_id GROUP BY 1,2;
