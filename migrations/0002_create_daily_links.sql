-- +goose Up
CREATE TABLE IF NOT EXISTS daily_clicks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  slug TEXT NOT NULL,
  day DATE NOT NULL,
  clicks INTEGER DEFAULT 0,
  UNIQUE(slug, day)
);

-- +goose Down
DROP TABLE IF EXISTS daily_clicks;