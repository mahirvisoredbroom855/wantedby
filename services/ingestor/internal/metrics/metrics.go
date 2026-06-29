// Package metrics registers all Prometheus metrics for the ingestor service.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// PostsFetched counts raw posts fetched per platform per run.
	PostsFetched = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ingestor_posts_fetched_total",
		Help: "Total posts fetched, labelled by platform.",
	}, []string{"platform"})

	// PostsQueued counts posts successfully pushed to the classifier queue.
	PostsQueued = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ingestor_posts_queued_total",
		Help: "Total posts pushed to queue:classify, labelled by platform.",
	}, []string{"platform"})

	// DedupHits counts posts skipped due to Redis deduplication.
	DedupHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ingestor_dedup_hits_total",
		Help: "Total posts skipped as duplicates, labelled by platform.",
	}, []string{"platform"})

	// FetchDuration measures wall-clock time per adapter fetch cycle.
	FetchDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ingestor_fetch_duration_seconds",
		Help:    "Duration of one full adapter fetch cycle.",
		Buckets: prometheus.DefBuckets,
	}, []string{"platform"})

	// APIErrors counts platform API errors by error type.
	APIErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ingestor_api_errors_total",
		Help: "Total platform API errors, labelled by platform and error type.",
	}, []string{"platform", "error_type"})
)
