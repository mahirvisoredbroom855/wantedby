// Package dedup provides Redis-backed deduplication for ingested posts.
// Key format: dedup:{platform}:{post_id} — TTL 30 days.
package dedup

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const ttl = 30 * 24 * time.Hour // 2592000s as per spec

// Deduplicator checks and marks posts as seen using a Redis SET per post.
type Deduplicator struct {
	rdb *redis.Client
}

// New constructs a Deduplicator backed by the given Redis client.
func New(rdb *redis.Client) *Deduplicator {
	return &Deduplicator{rdb: rdb}
}

// IsDuplicate returns true if the platform+id combination has been seen within the TTL window.
// If not seen, it marks the key so future calls return true.
func (d *Deduplicator) IsDuplicate(ctx context.Context, platform, postID string) (bool, error) {
	key := fmt.Sprintf("dedup:%s:%s", platform, postID)

	// SetNX returns true if the key was newly set (not a duplicate).
	set, err := d.rdb.SetNX(ctx, key, "1", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("redis setnx %s: %w", key, err)
	}
	return !set, nil // set=true means new; set=false means already existed
}
