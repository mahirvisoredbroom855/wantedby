package queue

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 2})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis not available, skipping: %v", err)
	}
	t.Cleanup(func() { rdb.FlushDB(context.Background()); rdb.Close() })
	return rdb
}

func TestPublisher_PublishWritesToQueue(t *testing.T) {
	rdb := newTestClient(t)
	p := New(rdb)

	ctx := context.Background()
	err := p.Publish(ctx, "6634f1a2b3c4d5e6f7a8b9c0", "reddit")
	require.NoError(t, err)

	// BRPOP from the right to match FIFO
	result, err := rdb.RPop(ctx, classifyQueue).Result()
	require.NoError(t, err)

	var job ClassifyJob
	require.NoError(t, json.Unmarshal([]byte(result), &job))

	assert.Equal(t, "6634f1a2b3c4d5e6f7a8b9c0", job.PostID)
	assert.Equal(t, "reddit", job.Platform)
	assert.Equal(t, 1, job.Priority)
}

func TestPublisher_MultiplePublishesOrderedFIFO(t *testing.T) {
	rdb := newTestClient(t)
	p := New(rdb)

	ctx := context.Background()
	ids := []string{"aaa", "bbb", "ccc"}
	for _, id := range ids {
		require.NoError(t, p.Publish(ctx, id, "hn"))
	}

	for _, expectedID := range ids {
		result, err := rdb.RPop(ctx, classifyQueue).Result()
		require.NoError(t, err)
		var job ClassifyJob
		require.NoError(t, json.Unmarshal([]byte(result), &job))
		assert.Equal(t, expectedID, job.PostID)
	}
}
