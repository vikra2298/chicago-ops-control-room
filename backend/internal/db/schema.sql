CREATE TABLE IF NOT EXISTS community_areas (
    id   INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS hourly_baselines (
    community_area_id INTEGER NOT NULL REFERENCES community_areas(id),
    weekday           INTEGER NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    hour              INTEGER NOT NULL CHECK (hour BETWEEN 0 AND 23),
    trip_count        INTEGER NOT NULL CHECK (trip_count >= 0),
    avg_fare_cents    INTEGER NOT NULL CHECK (avg_fare_cents >= 0),
    PRIMARY KEY (community_area_id, weekday, hour)
);

CREATE TABLE IF NOT EXISTS weather_snapshots (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    fetched_at        TEXT    NOT NULL,
    temp_c            REAL    NOT NULL,
    precipitation_mm  REAL    NOT NULL,
    weather_code      INTEGER NOT NULL,
    weather_label     TEXT    NOT NULL,
    source            TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_baselines_window
    ON hourly_baselines (weekday, hour);

CREATE INDEX IF NOT EXISTS idx_weather_fetched
    ON weather_snapshots (fetched_at DESC);
