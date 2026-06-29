// main is the entry point for the pain-ingestor service.
// It wires all adapters, stores, and publishers, then runs fetch cycles on internal tickers.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/wantedby/ingestor/internal/adapters"
	"github.com/wantedby/ingestor/internal/dedup"
	"github.com/wantedby/ingestor/internal/metrics"
	"github.com/wantedby/ingestor/internal/queue"
	"github.com/wantedby/ingestor/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	mongoURI := mustEnv("MONGO_URI")
	redisURL := mustEnv("REDIS_URL")
	redditClientID := mustEnv("REDDIT_CLIENT_ID")
	redditClientSecret := mustEnv("REDDIT_CLIENT_SECRET")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// MongoDB
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		logger.Error("mongo connect", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutCancel()
		if err := mongoClient.Disconnect(shutCtx); err != nil {
			logger.Error("mongo disconnect", "error", err)
		}
	}()

	mongoStore, err := store.New(ctx, mongoClient)
	if err != nil {
		logger.Error("mongo store init", "error", err)
		os.Exit(1)
	}

	// Redis
	redisOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		logger.Error("redis parse url", "error", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(redisOpts)
	defer rdb.Close()

	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Error("redis ping", "error", err)
		os.Exit(1)
	}

	deduplicator := dedup.New(rdb)
	publisher := queue.New(rdb)

	// Adapters
	redditAdapter := adapters.NewRedditAdapter(redditClientID, redditClientSecret, logger)
	hnAdapter := adapters.NewHNAdapter(logger)

	// HTTP server — /health and /metrics
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	go func() {
		logger.Info("http server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server", "error", err)
		}
	}()

	// Run one immediate fetch on startup, then on tickers
	runAdapter(ctx, logger, redditAdapter, mongoStore, deduplicator, publisher)
	runAdapter(ctx, logger, hnAdapter, mongoStore, deduplicator, publisher)

	redditTicker := time.NewTicker(15 * time.Minute)
	hnTicker := time.NewTicker(2 * time.Hour)
	defer redditTicker.Stop()
	defer hnTicker.Stop()

	logger.Info("ingestor running", "reddit_interval", "15m", "hn_interval", "2h")

	for {
		select {
		case <-ctx.Done():
			logger.Info("shutting down")
			shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutCancel()
			srv.Shutdown(shutCtx)
			return
		case <-redditTicker.C:
			go runAdapter(ctx, logger, redditAdapter, mongoStore, deduplicator, publisher)
		case <-hnTicker.C:
			go runAdapter(ctx, logger, hnAdapter, mongoStore, deduplicator, publisher)
		}
	}
}

// runAdapter executes one full fetch cycle for a single platform adapter.
func runAdapter(
	ctx context.Context,
	logger *slog.Logger,
	adapter adapters.PlatformAdapter,
	mongoStore *store.MongoStore,
	deduplicator *dedup.Deduplicator,
	publisher *queue.Publisher,
) {
	platform := adapter.Name()
	since := time.Now().Add(-16 * time.Minute) // slight overlap to avoid gaps on restarts

	logger.Info("fetch cycle starting", "platform", platform)
	timer := metrics.FetchDuration.WithLabelValues(platform)
	start := time.Now()

	posts, err := adapter.FetchSince(ctx, since)
	if err != nil {
		logger.Error("fetch failed", "platform", platform, "error", err)
		metrics.APIErrors.WithLabelValues(platform, "fetch_failed").Inc()
		return
	}

	timer.Observe(time.Since(start).Seconds())
	metrics.PostsFetched.WithLabelValues(platform).Add(float64(len(posts)))
	logger.Info("fetch cycle complete", "platform", platform, "posts", len(posts))

	queued := 0
	for _, post := range posts {
		isDup, err := deduplicator.IsDuplicate(ctx, post.Platform, post.PlatformID)
		if err != nil {
			logger.Error("dedup check failed", "platform_id", post.PlatformID, "error", err)
			continue
		}
		if isDup {
			metrics.DedupHits.WithLabelValues(platform).Inc()
			continue
		}

		id, err := mongoStore.Insert(ctx, post)
		if err != nil {
			logger.Error("mongo insert failed", "platform_id", post.PlatformID, "error", err)
			metrics.APIErrors.WithLabelValues(platform, "mongo_insert").Inc()
			continue
		}
		if id == "" {
			// duplicate key — already in mongo, skip silently
			continue
		}

		if err := mongoStore.UpdateStatus(ctx, id, "queued"); err != nil {
			logger.Error("status update failed", "id", id, "error", err)
		}

		if err := publisher.Publish(ctx, id, platform); err != nil {
			logger.Error("queue publish failed", "id", id, "error", err)
			metrics.APIErrors.WithLabelValues(platform, "queue_publish").Inc()
			continue
		}
		queued++
	}

	metrics.PostsQueued.WithLabelValues(platform).Add(float64(queued))
	logger.Info("cycle enqueued", "platform", platform, "queued", queued)
}

// mustEnv reads a required environment variable or exits.
func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("missing required env var", "key", key)
		os.Exit(1)
	}
	return v
}
