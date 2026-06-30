import Redis from 'ioredis';
import { config } from '../config';

export const redis = new Redis(config.redisUrl);

// Separate subscriber client — Redis requires a dedicated connection for pub/sub.
export const redisSub = new Redis(config.redisUrl);
