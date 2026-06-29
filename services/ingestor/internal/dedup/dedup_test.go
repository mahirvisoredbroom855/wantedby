package dedup

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient returns a Redis client pointed at the local test instance.
// Tests in this package require a running Redis on localhost:6379.
func newTestClient(t *testing.T) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 1})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis not available, skipping: %v", err)
	}
	t.Cleanup(func() { rdb.FlushDB(context.Background()); rdb.Close() })
	return rdb
}

func TestDeduplicator_FirstSeenReturnsFalse(t *testing.T) {
	rdb := newTestClient(t)
	d := New(rdb)

	isDup, err := d.IsDuplicate(context.Background(), "reddit", "t3_newpost")
	require.NoError(t, err)
	assert.False(t, isDup, "first time a post is seen must not be a duplicate")
}

func TestDeduplicator_SecondSeenReturnsTrue(t *testing.T) {
	rdb := newTestClient(t)
	d := New(rdb)

	ctx := context.Background()
	_, err := d.IsDuplicate(ctx, "reddit", "t3_repeat")
	require.NoError(t, err)

	isDup, err := d.IsDuplicate(ctx, "reddit", "t3_repeat")
	require.NoError(t, err)
	assert.True(t, isDup, "second call for same post must be a duplicate")
}

func TestDeduplicator_DifferentPlatformsSameIDNotDuplicate(t *testing.T) {
	rdb := newTestClient(t)
	d := New(rdb)

	ctx := context.Background()
	_, err := d.IsDuplicate(ctx, "reddit", "12345")
	require.NoError(t, err)

	isDup, err := d.IsDuplicate(ctx, "hn", "12345")
	require.NoError(t, err)
	assert.False(t, isDup, "same post ID on different platforms must not collide")
}
