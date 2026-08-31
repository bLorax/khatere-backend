package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	domainevent "yadegar/internal/domain/event"
	"yadegar/internal/telemetry"
)

// CachedEventRepository decorates domainevent.Repository. It caches only
// IsApprovedMember. Every other method passes straight through to inner.
type CachedEventRepository struct {
	inner domainevent.Repository
	cache *redis.Client
	ttl   time.Duration
}

func NewCachedEventRepository(inner domainevent.Repository, cache *redis.Client, ttl time.Duration) *CachedEventRepository {
	return &CachedEventRepository{inner: inner, cache: cache, ttl: ttl}
}

func membershipKey(eventID, userID string) string {
	return "membership:" + eventID + ":" + userID
}

// --- Pass-through methods (no caching) ---

func (r *CachedEventRepository) Create(ctx context.Context, e *domainevent.Event) error {
	return r.inner.Create(ctx, e)
}

func (r *CachedEventRepository) ListForUser(ctx context.Context, userID, search string) ([]domainevent.Event, error) {
	return r.inner.ListForUser(ctx, userID, search)
}

func (r *CachedEventRepository) Get(ctx context.Context, eventID string) (*domainevent.Event, error) {
	return r.inner.Get(ctx, eventID)
}

func (r *CachedEventRepository) ListMembers(ctx context.Context, eventID string) ([]domainevent.Member, error) {
	return r.inner.ListMembers(ctx, eventID)
}

func (r *CachedEventRepository) MemberStatus(ctx context.Context, eventID, userID string) (domainevent.MemberStatus, bool, error) {
	return r.inner.MemberStatus(ctx, eventID, userID)
}

// --- Cached method ---

func (r *CachedEventRepository) IsApprovedMember(ctx context.Context, eventID, userID string) (bool, error) {
	key := membershipKey(eventID, userID)

	cached, err := r.cache.Get(ctx, key).Result()
	if err == nil {
		telemetry.CacheHitsTotal.WithLabelValues("membership").Inc()
		return cached == "1", nil
	}
	telemetry.CacheMissesTotal.WithLabelValues("membership").Inc()

	approved, err := r.inner.IsApprovedMember(ctx, eventID, userID)
	if err != nil {
		return false, err
	}

	val := "0"
	if approved {
		val = "1"
	}
	// Best-effort cache write. A failed write here should not fail the request.
	_ = r.cache.Set(ctx, key, val, r.ttl).Err()

	return approved, nil
}

// --- Write methods (pass through, then invalidate the membership cache) ---

func (r *CachedEventRepository) AddMember(ctx context.Context, m *domainevent.Member) error {
	if err := r.inner.AddMember(ctx, m); err != nil {
		return err
	}
	_ = r.cache.Del(ctx, membershipKey(m.EventID, m.UserID)).Err()
	return nil
}

func (r *CachedEventRepository) ApproveMember(ctx context.Context, memberID, userID string) (eventID, taggedBy string, err error) {
	eventID, taggedBy, err = r.inner.ApproveMember(ctx, memberID, userID)
	if err != nil {
		return "", "", err
	}
	_ = r.cache.Del(ctx, membershipKey(eventID, userID)).Err()
	return eventID, taggedBy, nil
}

func (r *CachedEventRepository) RejectMember(ctx context.Context, memberID, userID string) (eventID, taggedBy string, err error) {
	eventID, taggedBy, err = r.inner.RejectMember(ctx, memberID, userID)
	if err != nil {
		return "", "", err
	}
	_ = r.cache.Del(ctx, membershipKey(eventID, userID)).Err()
	return eventID, taggedBy, nil
}

func (r *CachedEventRepository) RemoveMember(ctx context.Context, memberID, userID string) (string, error) {
	eventID, err := r.inner.RemoveMember(ctx, memberID, userID)
	if err != nil {
		return "", err
	}
	_ = r.cache.Del(ctx, membershipKey(eventID, userID)).Err()
	return eventID, nil
}
