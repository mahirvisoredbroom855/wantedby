-- Tables populated by the nightly clustering job (Issue #3).
-- users/subscriptions/api_keys/saved_clusters/alert_configs belong to Issue #4 (API + auth).

CREATE TABLE IF NOT EXISTS clusters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    category TEXT NOT NULL,
    gap_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    frequency INTEGER NOT NULL DEFAULT 0,
    trend_delta DOUBLE PRECISION NOT NULL DEFAULT 0,
    intensity_avg DOUBLE PRECISION NOT NULL DEFAULT 0,
    representative_quote TEXT NOT NULL DEFAULT '',
    implied_solution TEXT NOT NULL DEFAULT '',
    target_user TEXT NOT NULL DEFAULT '',
    first_seen TIMESTAMPTZ NOT NULL,
    last_seen TIMESTAMPTZ NOT NULL,
    post_count INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_clusters_category ON clusters (category);
CREATE INDEX IF NOT EXISTS idx_clusters_gap_score ON clusters (gap_score DESC);

CREATE TABLE IF NOT EXISTS cluster_posts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_id UUID NOT NULL REFERENCES clusters (id) ON DELETE CASCADE,
    mongo_classified_id TEXT NOT NULL,
    platform TEXT NOT NULL,
    pain_summary TEXT NOT NULL,
    intensity INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cluster_posts_cluster_id ON cluster_posts (cluster_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cluster_posts_mongo_id ON cluster_posts (mongo_classified_id);

CREATE TABLE IF NOT EXISTS competitors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_id UUID NOT NULL REFERENCES clusters (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    url TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL CHECK (source IN ('producthunt', 'github', 'appstore')),
    found_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_competitors_cluster_id ON competitors (cluster_id);
