-- settings exists since 0001 as (key, value); 0 marks rows written before this migration.
ALTER TABLE settings ADD COLUMN updated_at INTEGER NOT NULL DEFAULT 0;
CREATE TABLE provider_connections (
    provider TEXT PRIMARY KEY CHECK (provider IN ('brewfather')),
    user_id TEXT NOT NULL,
    api_key_sealed TEXT NOT NULL,
    connected_at INTEGER NOT NULL
);
CREATE TABLE sync_runs (
    provider TEXT PRIMARY KEY,
    started_at INTEGER NOT NULL,
    finished_at INTEGER,
    ok INTEGER,
    message TEXT,
    updated INTEGER NOT NULL DEFAULT 0,
    imported INTEGER NOT NULL DEFAULT 0,
    not_found INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE import_dismissed (
    source TEXT NOT NULL,
    ref TEXT NOT NULL,
    dismissed_at INTEGER NOT NULL,
    PRIMARY KEY (source, ref)
);
ALTER TABLE beers ADD COLUMN synced_at INTEGER;
ALTER TABLE beers ADD COLUMN sync_not_found INTEGER NOT NULL DEFAULT 0;
ALTER TABLE beers ADD COLUMN sync_archive_pending INTEGER NOT NULL DEFAULT 0;
ALTER TABLE beers ADD COLUMN sync_batch_archived INTEGER NOT NULL DEFAULT 0;
CREATE UNIQUE INDEX beers_source_ref ON beers(source, source_ref) WHERE source_ref IS NOT NULL;
