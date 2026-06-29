// Package queue publishes classify job IDs to the Redis LIST consumed by the classifier.
package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const classifyQueue = "queue:classify"

// ClassifyJob is the payload pushed to the Redis queue.
type ClassifyJob struct {
	PostID   string `json:"post_id"`
	Platform string `json:"platform"`
	Priority int    `json:"priority"`
}

// Publisher pushes classify jobs onto the Redis LIST.
type Publisher struct {
	rdb *redis.Client
}

// New constructs a Publisher.
func New(rdb *redis.Client) *Publisher {
	return &Publisher{rdb: rdb}
}

// Publish pushes a single classify job to the queue.
// LPUSH is used so the classifier's BRPOP (from the right) processes in FIFO order.
func (p *Publisher) Publish(ctx context.Context, postID, platform string) error {
	job := ClassifyJob{
		PostID:   postID,
		Platform: platform,
		Priority: 1,
	}

	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}

	if err := p.rdb.LPush(ctx, classifyQueue, payload).Err(); err != nil {
		return fmt.Errorf("lpush queue:classify: %w", err)
	}
	return nil
}
