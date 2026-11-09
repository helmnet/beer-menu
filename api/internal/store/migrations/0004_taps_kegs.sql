CREATE TABLE taps (
    id INTEGER PRIMARY KEY,
    position INTEGER NOT NULL,
    label TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0,1)),
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE TABLE kegs (
    id INTEGER PRIMARY KEY,
    beer_id INTEGER NOT NULL REFERENCES beers(id),
    type TEXT NOT NULL DEFAULT 'corny_5_gal',
    size_l REAL NOT NULL,
    remaining_l REAL NOT NULL,
    tap_id INTEGER REFERENCES taps(id) ON DELETE SET NULL,
    tap_label TEXT,
    tapped_at INTEGER,
    kicked_at INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX kegs_one_per_tap ON kegs(tap_id) WHERE kicked_at IS NULL AND tap_id IS NOT NULL;
CREATE INDEX kegs_beer ON kegs(beer_id);
INSERT INTO taps (position, label, enabled, created_at, updated_at) VALUES (1, 'Tap 1', 1, unixepoch(), unixepoch());
