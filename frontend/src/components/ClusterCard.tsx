import { Bookmark, BookmarkCheck, TrendingUp, Users, Zap } from 'lucide-react';
import { useMutation } from '@apollo/client/react';
import { GapScoreBar } from './GapScoreBar';
import { SAVE_CLUSTER, UNSAVE_CLUSTER, GET_CLUSTERS } from '../gql/queries';
import type { Cluster } from '../types';

interface Props {
  cluster: Cluster;
  onClick?: () => void;
}

export function ClusterCard({ cluster, onClick }: Props) {
  const [saveCluster, { loading: saving }] = useMutation(SAVE_CLUSTER, {
    refetchQueries: [GET_CLUSTERS],
  });
  const [unsaveCluster, { loading: unsaving }] = useMutation(UNSAVE_CLUSTER, {
    refetchQueries: [GET_CLUSTERS],
  });

  function handleBookmark(e: React.MouseEvent) {
    e.stopPropagation();
    if (cluster.isSaved) {
      unsaveCluster({ variables: { clusterId: cluster.id } });
    } else {
      saveCluster({ variables: { clusterId: cluster.id } });
    }
  }

  return (
    <div className="cluster-card" onClick={onClick} role="button" tabIndex={0}
      onKeyDown={(e) => e.key === 'Enter' && onClick?.()}>
      <div className="cluster-card-header">
        <span className="cluster-category">{cluster.category}</span>
        <button
          className="bookmark-btn"
          onClick={handleBookmark}
          disabled={saving || unsaving}
          aria-label={cluster.isSaved ? 'Unsave cluster' : 'Save cluster'}
        >
          {cluster.isSaved
            ? <BookmarkCheck size={16} color="#E8853A" />
            : <Bookmark size={16} />}
        </button>
      </div>

      <h3 className="cluster-name">{cluster.name}</h3>

      <div className="cluster-gap">
        <span className="cluster-gap-label">Gap Score</span>
        <GapScoreBar score={cluster.gapScore} />
      </div>

      <div className="cluster-stats">
        <div className="stat">
          <Users size={12} />
          <span>{cluster.postCount} posts</span>
        </div>
        <div className="stat">
          <Zap size={12} />
          <span>{cluster.intensityAvg.toFixed(1)} intensity</span>
        </div>
        <div className="stat">
          <TrendingUp size={12} />
          <span>{cluster.trendScore.toFixed(1)} trend</span>
        </div>
      </div>
    </div>
  );
}
