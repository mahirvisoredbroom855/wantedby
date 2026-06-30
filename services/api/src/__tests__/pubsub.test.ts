import { SimplePubSub } from '../websocket/pubsub';

describe('SimplePubSub', () => {
  it('delivers a published message to an active subscriber', async () => {
    const ps = new SimplePubSub();
    const iter = ps.asyncIterableIterator<{ x: number }>('TOPIC');

    ps.publish('TOPIC', { x: 42 });

    const result = await iter.next();
    expect(result.done).toBe(false);
    expect(result.value).toEqual({ x: 42 });
    await iter.return!();
  });

  it('queues messages published before next() is called', async () => {
    const ps = new SimplePubSub();
    const iter = ps.asyncIterableIterator<number>('T');

    ps.publish('T', 1);
    ps.publish('T', 2);

    expect((await iter.next()).value).toBe(1);
    expect((await iter.next()).value).toBe(2);
    await iter.return!();
  });

  it('marks done after return() is called', async () => {
    const ps = new SimplePubSub();
    const iter = ps.asyncIterableIterator<number>('T');
    const returnResult = await iter.return!();
    expect(returnResult.done).toBe(true);
  });
});
