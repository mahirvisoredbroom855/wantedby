import { useEffect, useRef } from 'react';
import { useSubscription } from '@apollo/client/react';
import { Wifi, WifiOff } from 'lucide-react';
import { LIVE_FEED_SUBSCRIPTION } from '../gql/queries';
import type { LivePost } from '../types';

const MAX_ITEMS = 50;

export function LiveFeed() {
  const { data, loading, error } = useSubscription(LIVE_FEED_SUBSCRIPTION);
  const feedRef = useRef<HTMLDivElement>(null);
  const itemsRef = useRef<LivePost[]>([]);

  useEffect(() => {
    const d = data as { liveFeed?: LivePost } | undefined;
    if (!d?.liveFeed) return;
    const post: LivePost = d.liveFeed;
    itemsRef.current = [post, ...itemsRef.current].slice(0, MAX_ITEMS);

    if (feedRef.current) {
      const el = feedRef.current;
      const item = document.createElement('div');
      item.className = `feed-item ${post.isPainSignal ? 'pain' : 'noise'}`;
      item.innerHTML = `
        <div class="feed-item-meta">
          <span class="feed-platform">${post.platform}</span>
          ${post.category ? `<span class="feed-category">${post.category}</span>` : ''}
          ${post.intensity ? `<span class="feed-intensity">⚡ ${post.intensity}</span>` : ''}
        </div>
        <p class="feed-title">${escapeHtml(post.title)}</p>
        ${post.painSummary ? `<p class="feed-summary">${escapeHtml(post.painSummary)}</p>` : ''}
      `;
      el.prepend(item);
      // Remove oldest if over limit
      while (el.children.length > MAX_ITEMS) {
        el.lastChild?.remove();
      }
    }
  }, [data]);

  return (
    <div className="live-feed-container">
      <div className="live-feed-header">
        <h2>Live Feed</h2>
        <div className={`live-status ${error ? 'error' : loading ? 'connecting' : 'connected'}`}>
          {error ? <WifiOff size={14} /> : <Wifi size={14} />}
          <span>{error ? 'Disconnected' : loading ? 'Connecting…' : 'Live'}</span>
        </div>
      </div>
      <div ref={feedRef} className="feed-items" />
    </div>
  );
}

function escapeHtml(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}
