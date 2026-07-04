import { redisSub } from '../db/redis';
import { pubsub, LIVE_FEED_TOPIC, CLUSTER_UPDATED_TOPIC } from './pubsub';
import { config } from '../config';

// Bridge Redis Pub/Sub into the GraphQL PubSub so all WebSocket subscribers
// receive messages published by the classifier service.
export function startRedisSubscriber(): void {
  redisSub.subscribe(config.liveFeedChannel, config.clusterUpdatesChannel, (err) => {
    if (err) {
      console.error('Redis subscribe error', err);
    }
  });

  redisSub.on('message', (channel: string, message: string) => {
    try {
      const payload = JSON.parse(message);
      if (channel === config.liveFeedChannel) {
        pubsub.publish(LIVE_FEED_TOPIC, { liveFeed: payload });
      } else if (channel === config.clusterUpdatesChannel) {
        pubsub.publish(CLUSTER_UPDATED_TOPIC, { clusterUpdated: payload });
      }
    } catch {
      // Malformed messages are silently dropped — don't crash the process.
    }
  });
}
