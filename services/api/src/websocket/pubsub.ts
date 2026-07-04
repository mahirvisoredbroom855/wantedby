// Minimal PubSub that exposes the asyncIterableIterator interface required
// by graphql-ws subscriptions, backed by Node.js EventEmitter.
import { EventEmitter } from 'events';

export const LIVE_FEED_TOPIC = 'LIVE_FEED';
export const CLUSTER_UPDATED_TOPIC = 'CLUSTER_UPDATED';

export class SimplePubSub {
  private emitter = new EventEmitter();

  publish(topic: string, payload: unknown): void {
    this.emitter.emit(topic, payload);
  }

  asyncIterableIterator<T>(topic: string): AsyncIterableIterator<T> {
    const queue: T[] = [];
    const waiting: Array<(value: IteratorResult<T>) => void> = [];
    let done = false;

    const listener = (payload: T) => {
      if (waiting.length) {
        waiting.shift()!({ value: payload, done: false });
      } else {
        queue.push(payload);
      }
    };

    const emitter = this.emitter;
    emitter.on(topic, listener);

    return {
      [Symbol.asyncIterator]() {
        return this;
      },
      next(): Promise<IteratorResult<T>> {
        if (queue.length) return Promise.resolve({ value: queue.shift()!, done: false });
        if (done) return Promise.resolve({ value: undefined as unknown as T, done: true });
        return new Promise((resolve) => waiting.push(resolve));
      },
      return(): Promise<IteratorResult<T>> {
        done = true;
        emitter.off(topic, listener);
        return Promise.resolve({ value: undefined as unknown as T, done: true });
      },
    } as AsyncIterableIterator<T>;
  }
}

export const pubsub = new SimplePubSub();
