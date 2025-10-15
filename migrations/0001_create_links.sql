-- +goose Up
CREATE TABLE IF NOT EXISTS links (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  slug TEXT NOT NULL UNIQUE,     -- short code like "abc123"
  url TEXT NOT NULL,             -- destination URL
  user TEXT DEFAULT NULL,                     -- optional creator id/name
  created_at DATETIME NOT NULL DEFAULT (datetime('now')),
  clicks INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_links_slug ON links(slug);
-- +goose Down
DROP TABLE IF EXISTS links;
