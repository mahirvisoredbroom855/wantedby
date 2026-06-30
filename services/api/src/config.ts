import 'dotenv/config';

function required(name: string): string {
  const val = process.env[name];
  if (!val) throw new Error(`Missing required env var: ${name}`);
  return val;
}

function optional(name: string, fallback = ''): string {
  return process.env[name] ?? fallback;
}

export const config = {
  port: parseInt(optional('PORT', '4000'), 10),
  nodeEnv: optional('NODE_ENV', 'development'),

  mongoUri: required('MONGO_URI'),
  databaseUrl: required('DATABASE_URL'),
  redisUrl: required('REDIS_URL'),

  auth0Domain: required('AUTH0_DOMAIN'),
  auth0Audience: required('AUTH0_AUDIENCE'),

  stripeSecretKey: optional('STRIPE_SECRET_KEY'),
  stripeWebhookSecret: optional('STRIPE_WEBHOOK_SECRET'),

  resendApiKey: optional('RESEND_API_KEY'),
  resendFromEmail: optional('RESEND_FROM_EMAIL', 'noreply@wantedby.io'),

  liveFeedChannel: 'live:posts',
  clusterUpdatesChannel: 'live:clusters',
} as const;
