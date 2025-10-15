-- name: AddLink :one
INSERT INTO links (slug, url, user, created_at, clicks)
VALUES (:slug, :url, :user, datetime('now'), 0)
RETURNING id, slug, url, user, created_at, clicks;

-- name: GetLink :one
SELECT id, slug, url, user, created_at, clicks
FROM links
WHERE slug = ?;

-- name: AddClick :exec
UPDATE links SET clicks = clicks + ? WHERE slug = ?;

-- name: SaveDailyClicks :exec
INSERT INTO daily_clicks (slug, day, clicks)
VALUES (?, date('now'), ?)
ON CONFLICT(slug, day) DO UPDATE SET clicks = clicks + excluded.clicks;

-- name: GetLinkStats :one
SELECT id, slug, url, user, created_at, clicks
FROM links
WHERE slug = :slug;

-- name: GetDailyClicks :many
SELECT day, clicks
FROM daily_clicks
WHERE slug = :slug
  AND day >= date('now','-6 days')
ORDER BY day ASC;