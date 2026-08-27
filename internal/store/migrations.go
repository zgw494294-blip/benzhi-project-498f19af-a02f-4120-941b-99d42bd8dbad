package store

const schemaVersion1 = `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS cases (
  id TEXT PRIMARY KEY,
  cave_code TEXT NOT NULL,
  mural_zone TEXT NOT NULL,
  status TEXT NOT NULL,
  version INTEGER NOT NULL,
  aggregate_json BLOB NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_cases_updated ON cases(updated_at DESC);
CREATE TABLE IF NOT EXISTS evidence_revisions (
  id TEXT PRIMARY KEY,
  case_id TEXT NOT NULL REFERENCES cases(id),
  zone_code TEXT NOT NULL,
  revision INTEGER NOT NULL,
  evidence_json BLOB NOT NULL,
  recorded_at TEXT NOT NULL,
  UNIQUE(case_id, zone_code, revision)
);
CREATE TABLE IF NOT EXISTS treatment_plans (
  id TEXT PRIMARY KEY,
  case_id TEXT NOT NULL REFERENCES cases(id),
  version INTEGER NOT NULL,
  plan_json BLOB NOT NULL,
  submitted_at TEXT NOT NULL,
  UNIQUE(case_id, version)
);
CREATE TABLE IF NOT EXISTS pilot_trials (
  id TEXT PRIMARY KEY,
  case_id TEXT NOT NULL REFERENCES cases(id),
  revision INTEGER NOT NULL,
  trial_json BLOB NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE(case_id, revision)
);
CREATE TABLE IF NOT EXISTS frozen_manifests (
  case_id TEXT PRIMARY KEY REFERENCES cases(id),
  digest TEXT NOT NULL UNIQUE,
  manifest_json BLOB NOT NULL,
  frozen_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS credentials (
  credential_no TEXT PRIMARY KEY,
  case_id TEXT NOT NULL UNIQUE REFERENCES cases(id),
  credential_json BLOB NOT NULL,
  issued_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS idempotency_results (
  idempotency_key TEXT PRIMARY KEY,
  case_id TEXT NOT NULL,
  operation TEXT NOT NULL,
  response_json BLOB NOT NULL,
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS audit_events (
  case_id TEXT NOT NULL REFERENCES cases(id),
  sequence INTEGER NOT NULL,
  event_type TEXT NOT NULL,
  actor TEXT NOT NULL,
  summary TEXT NOT NULL,
  occurred_at TEXT NOT NULL,
  case_version INTEGER NOT NULL,
  previous_hash TEXT NOT NULL,
  event_hash TEXT NOT NULL,
  details_json BLOB NOT NULL,
  PRIMARY KEY(case_id, sequence)
);
CREATE TRIGGER IF NOT EXISTS evidence_immutable_update
BEFORE UPDATE ON evidence_revisions BEGIN SELECT RAISE(ABORT, 'evidence revisions are immutable'); END;
CREATE TRIGGER IF NOT EXISTS evidence_immutable_delete
BEFORE DELETE ON evidence_revisions BEGIN SELECT RAISE(ABORT, 'evidence revisions are immutable'); END;
CREATE TRIGGER IF NOT EXISTS plan_immutable_update
BEFORE UPDATE ON treatment_plans BEGIN SELECT RAISE(ABORT, 'treatment plans are immutable'); END;
CREATE TRIGGER IF NOT EXISTS trial_immutable_update
BEFORE UPDATE ON pilot_trials BEGIN SELECT RAISE(ABORT, 'pilot trials are immutable'); END;
`
