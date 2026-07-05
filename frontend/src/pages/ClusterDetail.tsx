import { useParams, useNavigate } from 'react-router-dom';
import { useQuery } from '@apollo/client/react';
import { ArrowLeft, ExternalLink } from 'lucide-react';
import { GapScoreBar } from '../components/GapScoreBar';
import { TrendChart } from '../components/TrendChart';
import { GET_CLUSTER } from '../gql/queries';
import type { Cluster, ClusterPost, Competitor } from '../types';

export function ClusterDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { data, loading, error } = useQuery(GET_CLUSTER, { variables: { id } });

  if (loading) return <div className="detail-loading">Loading…</div>;
  if (error) return <div className="error-banner">{error.message}</div>;
  const d = data as { cluster?: Cluster & { posts: ClusterPost[]; competitors: Competitor[] } } | undefined;
  if (!d?.cluster) return <div className="error-banner">Cluster not found</div>;

  const cluster = d.cluster;

  return (
    <div className="cluster-detail">
      <button className="back-btn" onClick={() => navigate(-1)}>
        <ArrowLeft size={14} /> Back
      </button>

      <div className="detail-header">
        <span className="cluster-category">{cluster.category}</span>
        <h1>{cluster.name}</h1>
        <div className="detail-gap">
          <span>Gap Score</span>
          <GapScoreBar score={cluster.gapScore} size="md" />
        </div>
      </div>

      <div className="detail-stats-row">
        <div className="stat-box">
          <span className="stat-label">Posts</span>
          <span className="stat-value">{cluster.postCount}</span>
        </div>
        <div className="stat-box">
          <span className="stat-label">Intensity</span>
          <span className="stat-value">{cluster.intensityAvg.toFixed(1)}</span>
        </div>
        <div className="stat-box">
          <span className="stat-label">Trend</span>
          <span className="stat-value">{cluster.trendScore.toFixed(1)}</span>
        </div>
        <div className="stat-box">
          <span className="stat-label">Competitor Gap</span>
          <span className="stat-value">{cluster.competitorGap.toFixed(1)}</span>
        </div>
      </div>

      <section className="detail-section">
        <h2>Trend</h2>
        <TrendChart data={[]} />
      </section>

      {cluster.competitors.length > 0 && (
        <section className="detail-section">
          <h2>Existing Solutions</h2>
          <div className="competitor-list">
            {cluster.competitors.map((c: Competitor) => (
              <div key={c.id} className="competitor-row">
                <span className="competitor-name">{c.name}</span>
                <span className="competitor-source">{c.source}</span>
                {c.url && (
                  <a href={c.url} target="_blank" rel="noreferrer" className="competitor-link">
                    <ExternalLink size={12} />
                  </a>
                )}
              </div>
            ))}
          </div>
        </section>
      )}

      <section className="detail-section">
        <h2>Posts ({cluster.posts.length})</h2>
        <div className="post-list">
          {cluster.posts.map((post: ClusterPost) => (
            <div key={post.id} className="post-row">
              <div className="post-meta">
                <span className="post-platform">{post.platform}</span>
                {post.intensity && <span className="post-intensity">⚡ {post.intensity}</span>}
                {post.score > 0 && <span className="post-score">▲ {post.score}</span>}
              </div>
              <p className="post-title">
                {post.url ? (
                  <a href={post.url} target="_blank" rel="noreferrer">{post.title}</a>
                ) : post.title}
              </p>
              {post.painSummary && <p className="post-summary">{post.painSummary}</p>}
            </div>
          ))}
          {cluster.posts.length === 0 && (
            <p className="empty-state">No posts yet.</p>
          )}
        </div>
      </section>
    </div>
  );
}
