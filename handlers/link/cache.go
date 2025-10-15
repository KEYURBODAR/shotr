package link

import (
	"context"
)

// resolveURL looks up a slug from cache or DB.
// Returns (url, cacheHit, error)
func (l *Link) resolveURL(ctx context.Context, slug string) (string, bool, error) {
	if l.Cache != nil {
		if v, ok := l.Cache.Get(slug); ok {
			if s, ok := v.(string); ok {
				return s, true, nil
			}
			l.Cache.Remove(slug) // type mismatch — evict
		}
	}

	linkRow, err := l.Q.GetLink(ctx, slug)
	if err != nil {
		return "", false, err
	}

	if l.Cache != nil {
		l.Cache.Add(slug, linkRow.Url)
	}
	return linkRow.Url, false, nil
}