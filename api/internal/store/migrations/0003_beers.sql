CREATE TABLE beers (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    style TEXT,
    abv REAL,
    ibu INTEGER,
    srm REAL,
    description TEXT,
    brewer TEXT,
    brew_date TEXT,
    status TEXT NOT NULL DEFAULT 'ready' CHECK (status IN ('planned','fermenting','conditioning','ready')),
    source TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('manual','brewfather')),
    source_ref TEXT,
    archived_at INTEGER,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
