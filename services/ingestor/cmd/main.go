// main is the entry point for the pain-ingestor service.
// It wires all adapters, stores, and publishers, then runs fetch cycles on internal tickers.
//
// Reddit is intentionally excluded from the automatic ticker: it currently runs
// against a metered third-party API (RapidAPI Reddit34, Basic/free tier, 50
// requests/month) while the official Reddit Data Access Request is pending.
// A 15-minute ticker would exhaust the entire monthly quota in under an hour.
// Pass -reddit-once to run a single manual Reddit fetch cycle for testing.
package main

import (
	"context"
	"flag"
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
	redditOnce := flag.Bool("reddit-once", false, "run a single manual Reddit fetch cycle, then exit")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	mongoURI := mustEnv("MONGO_URI")
	redisURL := mustEnv("REDIS_URL")
	redditAPIKey := mustEnv("REDDIT_RAPIDAPI_KEY")
	redditAPIHost := mustEnv("REDDIT_RAPIDAPI_HOST")

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

	mongoStore, err := store.New(ctx, mongoClient, "wantedby")
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
	redditAdapter := adapters.NewRedditAdapter(redditAPIKey, redditAPIHost, logger)
	hnAdapter := adapters.NewHNAdapter(logger)

	// -reddit-once: run a single manual Reddit fetch and exit. Does not start the
	// HTTP server or any tickers — this is a one-shot CLI test, not the service.
	if *redditOnce {
		logger.Info("running single manual reddit fetch cycle")
		runAdapter(ctx, logger, redditAdapter, mongoStore, deduplicator, publisher)
		logger.Info("manual reddit fetch cycle complete, exiting")
		return
	}

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

	// HN runs immediately and on its own ticker — free, unauthenticated, unlimited.
	// Reddit does NOT run here automatically; see -reddit-once above.
	runAdapter(ctx, logger, hnAdapter, mongoStore, deduplicator, publisher)

	hnTicker := time.NewTicker(2 * time.Hour)
	defer hnTicker.Stop()

	logger.Info("ingestor running", "hn_interval", "2h", "reddit", "manual only (-reddit-once)")

	for {
		select {
		case <-ctx.Done():
			logger.Info("shutting down")
			shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutCancel()
			srv.Shutdown(shutCtx)
			return
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
