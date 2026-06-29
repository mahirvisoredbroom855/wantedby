// Package store handles writing raw posts to MongoDB.
package store

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/wantedby/ingestor/internal/adapters"
)

const collectionName = "raw_posts"

// RawPostDocument is the MongoDB document shape for raw_posts collection.
type RawPostDocument struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	PlatformID   string             `bson:"platform_id"`
	Platform     string             `bson:"platform"`
	Title        string             `bson:"title"`
	Body         string             `bson:"body"`
	Author       string             `bson:"author"`
	URL          string             `bson:"url"`
	Subreddit    string             `bson:"subreddit"`
	Score        int                `bson:"score"`
	CommentCount int                `bson:"comment_count"`
	CreatedAt    time.Time          `bson:"created_at"`
	FetchedAt    time.Time          `bson:"fetched_at"`
	Status       string             `bson:"status"` // "pending" on insert
	Raw          map[string]any     `bson:"raw"`
}

// MongoStore writes raw posts to MongoDB and returns the inserted document ID.
type MongoStore struct {
	coll *mongo.Collection
}

// New constructs a MongoStore against the given database name and ensures the
// required indexes exist. Production uses "wantedby"; tests use an isolated
// database name so they never touch real dev data.
func New(ctx context.Context, client *mongo.Client, dbName string) (*MongoStore, error) {
	coll := client.Database(dbName).Collection(collectionName)

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "platform_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: "created_at", Value: -1}}},
		{Keys: bson.D{{Key: "cluster_id", Value: 1}}},
	}

	if _, err := coll.Indexes().CreateMany(ctx, indexes); err != nil {
		return nil, fmt.Errorf("create indexes: %w", err)
	}

	return &MongoStore{coll: coll}, nil
}

// Insert writes one RawPost to MongoDB. Returns the new ObjectID as a hex string.
// If a document with the same platform_id already exists (duplicate key), returns an empty
// string and no error — the dedup layer should have prevented this, but we handle it safely.
func (s *MongoStore) Insert(ctx context.Context, post adapters.RawPost) (string, error) {
	doc := RawPostDocument{
		PlatformID:   post.PlatformID,
		Platform:     post.Platform,
		Title:        post.Title,
		Body:         post.Body,
		Author:       post.Author,
		URL:          post.URL,
		Subreddit:    post.Subreddit,
		Score:        post.Score,
		CommentCount: post.CommentCount,
		CreatedAt:    post.CreatedAt,
		FetchedAt:    post.FetchedAt,
		Status:       "pending",
		Raw:          post.Raw,
	}

	result, err := s.coll.InsertOne(ctx, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return "", nil
		}
		return "", fmt.Errorf("insert raw post: %w", err)
	}

	oid, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return "", fmt.Errorf("unexpected inserted id type: %T", result.InsertedID)
	}
	return oid.Hex(), nil
}

// UpdateStatus sets the status field on a document identified by its ObjectID hex string.
func (s *MongoStore) UpdateStatus(ctx context.Context, id, status string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("invalid object id %q: %w", id, err)
	}
	_, err = s.coll.UpdateByID(ctx, oid, bson.M{"$set": bson.M{"status": status}})
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	return nil
}
