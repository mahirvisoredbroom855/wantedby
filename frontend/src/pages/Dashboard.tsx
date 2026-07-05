import { useState } from 'react';
import { useQuery } from '@apollo/client/react';
import { useNavigate } from 'react-router-dom';
import { Search, SlidersHorizontal } from 'lucide-react';
import { ClusterCard } from '../components/ClusterCard';
import { LiveFeed } from '../components/LiveFeed';
import { GET_CLUSTERS } from '../gql/queries';
import type { Cluster } from '../types';

const CATEGORIES = ['All', 'devtools', 'finance', 'health', 'legal', 'productivity', 'ecommerce'];

export function Dashboard() {
  const navigate = useNavigate();
  const [category, setCategory] = useState<string | undefined>(undefined);
  const [minGapScore, setMinGapScore] = useState<number | undefined>(undefined);
  const [search, setSearch] = useState('');

  const { data, loading, error } = useQuery(GET_CLUSTERS, {
    variables: {
      filters: {
        category: category ?? undefined,
        minGapScore: minGapScore ?? undefined,
        limit: 40,
      },
    },
  });

  const clusters: Cluster[] = ((data as { clusters?: Cluster[] })?.clusters ?? []).filter((c: Cluster) =>
    search === '' || c.name.toLowerCase().includes(search.toLowerCase()),
  );

  return (
    <div className="dashboard">
      <div className="dashboard-main">
        <div className="dashboard-toolbar">
          <div className="search-box">
            <Search size={14} />
            <input
              placeholder="Search clusters…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>

          <div className="filter-row">
            {CATEGORIES.map((cat) => (
              <button
                key={cat}
                className={`category-pill ${(cat === 'All' ? !category : category === cat) ? 'active' : ''}`}
                onClick={() => setCategory(cat === 'All' ? undefined : cat)}
              >
                {cat}
              </button>
            ))}
          </div>

          <div className="gap-filter">
            <SlidersHorizontal size={13} />
            <label htmlFor="min-gap">Min gap score</label>
            <input
              id="min-gap"
              type="range"
              min={0}
              max={10}
              step={0.5}
              value={minGapScore ?? 0}
              onChange={(e) => setMinGapScore(parseFloat(e.target.value) || undefined)}
            />
            <span>{minGapScore?.toFixed(1) ?? 'Any'}</span>
          </div>
        </div>

        {error && (
          <div className="error-banner">Failed to load clusters: {error.message}</div>
        )}

        {loading ? (
          <div className="cluster-grid skeleton">
            {Array.from({ length: 8 }).map((_, i) => (
              <div key={i} className="cluster-card skeleton-card" />
            ))}
          </div>
        ) : (
          <>
            <p className="result-count">{clusters.length} clusters</p>
            <div className="cluster-grid">
              {clusters.map((cluster) => (
                <ClusterCard
                  key={cluster.id}
                  cluster={cluster}
                  onClick={() => navigate(`/clusters/${cluster.id}`)}
                />
              ))}
            </div>
            {clusters.length === 0 && (
              <div className="empty-state">No clusters match your filters.</div>
            )}
          </>
        )}
      </div>

      <aside className="dashboard-sidebar">
        <LiveFeed />
      </aside>
    </div>
  );
}
