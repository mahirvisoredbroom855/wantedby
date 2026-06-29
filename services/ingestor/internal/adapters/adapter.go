// Package adapters defines the platform-agnostic interface all source adapters must implement.
package adapters

import (
	"context"
	"time"
)

// PlatformAdapter is the contract every data source must satisfy.
// Adding a new source (Twitter, Dev.to) requires only a new file in this package.
type PlatformAdapter interface {
	// Name returns the platform identifier used in dedup keys and MongoDB documents.
	Name() string

	// FetchSince returns all posts published after the given time.
	// Implementations are responsible for their own rate limiting.
	FetchSince(ctx context.Context, since time.Time) ([]RawPost, error)
}

// RawPost is the normalised representation of a post from any platform.
// The Raw field preserves the full original API payload for forensic use.
type RawPost struct {
	PlatformID   string         // e.g. "reddit:t3_abc123" — used as dedup key suffix
	Platform     string         // "reddit" | "hn"
	Title        string
	Body         string
	Author       string
	URL          string
	Subreddit    string         // empty for HN
	Score        int            // upvotes / HN points
	CommentCount int
	CreatedAt    time.Time
	FetchedAt    time.Time
	Raw          map[string]any // full original API response
}
