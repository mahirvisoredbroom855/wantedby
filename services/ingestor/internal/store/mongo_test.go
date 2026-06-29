package store

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wantedby/ingestor/internal/adapters"
)

// newTestStore connects to a local MongoDB instance and returns a MongoStore
// backed by a uniquely-named test database, cleaned up after the test.
func newTestStore(t *testing.T) *MongoStore {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Skipf("mongodb not available, skipping: %v", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		t.Skipf("mongodb not reachable, skipping: %v", err)
	}

	testDBName := "wantedby_test"
	s, err := New(ctx, client, testDBName)
	require.NoError(t, err)

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		client.Database(testDBName).Drop(cleanupCtx)
		client.Disconnect(cleanupCtx)
	})

	return s
}

func samplePost(platformID string) adapters.RawPost {
	now := time.Now().UTC()
	return adapters.RawPost{
		PlatformID:   platformID,
		Platform:     "reddit",
		Title:        "I wish there was a tool that did X",
		Body:         "body text",
		Author:       "tester",
		URL:          "https://reddit.com/r/test/comments/abc",
		Subreddit:    "test",
		Score:        10,
		CommentCount: 2,
		CreatedAt:    now,
		FetchedAt:    now,
		Raw:          map[string]any{"id": "abc"},
	}
}

func TestMongoStore_InsertReturnsID(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, err := s.Insert(ctx, samplePost("reddit:t3_inserttest1"))
	require.NoError(t, err)
	assert.NotEmpty(t, id)
}

func TestMongoStore_InsertDuplicatePlatformIDReturnsEmptyNoError(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	post := samplePost("reddit:t3_duptest1")
	id, err := s.Insert(ctx, post)
	require.NoError(t, err)
	require.NotEmpty(t, id)

	dupID, err := s.Insert(ctx, post)
	require.NoError(t, err, "duplicate platform_id must not return an error")
	assert.Empty(t, dupID, "duplicate platform_id must return an empty id")
}

func TestMongoStore_UpdateStatusPersists(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, err := s.Insert(ctx, samplePost("reddit:t3_statustest1"))
	require.NoError(t, err)

	err = s.UpdateStatus(ctx, id, "queued")
	require.NoError(t, err)
}

func TestMongoStore_UpdateStatusInvalidIDErrors(t *testing.T) {
	s := newTestStore(t)
	err := s.UpdateStatus(context.Background(), "not-a-valid-object-id", "queued")
	assert.Error(t, err)
}
