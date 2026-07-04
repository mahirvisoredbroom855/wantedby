import http from 'http';
import express from 'express';
import cors from 'cors';
import helmet from 'helmet';
import { json, raw } from 'body-parser';
import { ApolloServer } from '@apollo/server';
import { expressMiddleware } from '@as-integrations/express5';
import { makeExecutableSchema } from '@graphql-tools/schema';
import { WebSocketServer } from 'ws';
import { useServer } from 'graphql-ws/lib/use/ws';
import pino from 'pino';
import pinoHttp from 'pino-http';

import { typeDefs } from './schema/typeDefs';
import { resolvers } from './schema/resolvers';
import { optionalAuth, AuthRequest, AuthUser } from './auth/middleware';
import { getOrCreateUser } from './schema/resolvers/users';
import { stripeRouter } from './webhooks/stripe';
import { startRedisSubscriber } from './websocket/liveFeed';
import { config } from './config';
import { pool } from './db/postgres';

const logger = pino({ level: 'info' });

async function main() {
  const app = express();

  app.use(helmet({ contentSecurityPolicy: config.nodeEnv === 'production' }));
  app.use(cors({ origin: true, credentials: true }));
  app.use(pinoHttp({ logger }));

  // Stripe webhook needs the raw body for signature verification — mount before json().
  app.use('/webhooks/stripe', raw({ type: 'application/json' }), stripeRouter);
  app.use(json());

  app.get('/health', (_req, res) => res.json({ status: 'ok' }));
  app.get('/ready', async (_req, res) => {
    try {
      await pool.query('SELECT 1');
      res.json({ status: 'ok' });
    } catch {
      res.status(503).json({ status: 'unavailable' });
    }
  });

  const schema = makeExecutableSchema({ typeDefs, resolvers });

  const apolloServer = new ApolloServer({
    schema,
    introspection: true,
  });
  await apolloServer.start();

  app.use(
    '/graphql',
    optionalAuth,
    expressMiddleware(apolloServer, {
      context: async ({ req }) => {
        const authReq = req as AuthRequest;
        let userDbId: string | undefined;

        if (authReq.user) {
          try {
            const dbUser = await getOrCreateUser(authReq.user as AuthUser);
            userDbId = dbUser.id;
          } catch {
            // Non-fatal: unauthenticated context still works for public queries.
          }
        }

        return { user: authReq.user, userDbId };
      },
    }),
  );

  const httpServer = http.createServer(app);

  // GraphQL subscriptions over WebSocket.
  const wss = new WebSocketServer({ server: httpServer, path: '/graphql' });
  useServer({ schema }, wss);

  // Bridge Redis Pub/Sub → GraphQL PubSub.
  startRedisSubscriber();

  httpServer.listen(config.port, () => {
    logger.info({ port: config.port }, 'API server ready');
  });
}

main().catch((err) => {
  console.error('Fatal startup error', err);
  process.exit(1);
});
