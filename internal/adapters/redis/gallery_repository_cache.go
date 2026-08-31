package redis

import (
	"context"
	"encoding/json"
	"time"
	"yadegar/internal/telemetry"

	"github.com/redis/go-redis/v9"

	domaingallery "yadegar/internal/domain/gallery"
)

// CachedGalleryRepository decorates domaingallery.Repository. It caches
// ListForUser with a short TTL. There is no invalidation logic here — a
// short TTL is enough, since Gallery tolerates being slightly stale.
type CachedGalleryRepository struct {
	inner domaingallery.Repository
	cache *redis.Client
	ttl   time.Duration
}

func NewCachedGalleryRepository(inner domaingallery.Repository, cache *redis.Client, ttl time.Duration) *CachedGalleryRepository {
	return &CachedGalleryRepository{inner: inner, cache: cache, ttl: ttl}
}

func galleryKey(userID string) string {
	return "gallery:" + userID
}

func (r *CachedGalleryRepository) ListForUser(ctx context.Context, userID string) ([]domaingallery.Event, error) {
	key := galleryKey(userID)

	cached, err := r.cache.Get(ctx, key).Result()
	if err == nil {
		var events []domaingallery.Event
		if jsonErr := json.Unmarshal([]byte(cached), &events); jsonErr == nil {
			telemetry.CacheHitsTotal.WithLabelValues("gallery").Inc()
			return events, nil
		}
		// Corrupt cache value: fall through and hit inner.
	}
	telemetry.CacheMissesTotal.WithLabelValues("gallery").Inc()

	events, err := r.inner.ListForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	if data, marshalErr := json.Marshal(events); marshalErr == nil {
		// Best-effort cache write. A failed write here should not fail the request.
		_ = r.cache.Set(ctx, key, data, r.ttl).Err()
	}

	return events, nil
}
