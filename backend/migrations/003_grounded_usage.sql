CREATE TABLE grounded_usage_days (day date PRIMARY KEY, accepted integer NOT NULL DEFAULT 0);
CREATE TABLE grounded_owner_usage_days (owner_id uuid NOT NULL REFERENCES owners(id), day date NOT NULL, accepted integer NOT NULL DEFAULT 0, PRIMARY KEY(owner_id,day));
