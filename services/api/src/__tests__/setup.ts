// Minimal env vars to satisfy config.ts required() checks in test environment.
process.env.MONGO_URI = 'mongodb://localhost:27017';
process.env.DATABASE_URL = 'postgresql://wantedby:wantedby@localhost:5433/wantedby_db';
process.env.REDIS_URL = 'redis://localhost:6379';
process.env.AUTH0_DOMAIN = 'test.auth0.com';
process.env.AUTH0_AUDIENCE = 'https://api.wantedby.io';
process.env.STRIPE_WEBHOOK_SECRET = 'whsec_test';
