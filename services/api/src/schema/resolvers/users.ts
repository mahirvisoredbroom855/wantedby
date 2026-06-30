import crypto from 'crypto';
import { pool } from '../../db/postgres';
import type { AuthUser } from '../../auth/middleware';

import type { GqlContext } from './clusters';

// Upsert by Auth0 sub, returning the db user row.
export async function getOrCreateUser(auth0User: AuthUser): Promise<{ id: string; plan: string }> {
  const { rows } = await pool.query(
    `INSERT INTO users (auth0_id, email, name)
     VALUES ($1, $2, $3)
     ON CONFLICT (auth0_id) DO UPDATE
       SET email = EXCLUDED.email, name = EXCLUDED.name, updated_at = now()
     RETURNING id, plan`,
    [auth0User.sub, auth0User.email ?? '', auth0User.name ?? null],
  );
  return rows[0];
}

export const userResolvers = {
  Query: {
    async me(_: unknown, __: unknown, ctx: GqlContext) {
      if (!ctx.user || !ctx.userDbId) return null;
      const { rows } = await pool.query('SELECT * FROM users WHERE id = $1', [ctx.userDbId]);
      return rows[0] ? mapUser(rows[0]) : null;
    },
  },

  User: {
    async savedClusters(parent: { id: string }) {
      const { rows } = await pool.query(
        `SELECT c.* FROM clusters c
         JOIN saved_clusters sc ON sc.cluster_id = c.id
         WHERE sc.user_id = $1
         ORDER BY sc.created_at DESC`,
        [parent.id],
      );
      return rows.map(mapClusterBrief);
    },

    async alertConfigs(parent: { id: string }) {
      const { rows } = await pool.query(
        'SELECT * FROM alert_configs WHERE user_id = $1 ORDER BY category',
        [parent.id],
      );
      return rows.map(mapAlert);
    },

    async apiKeys(parent: { id: string }) {
      const { rows } = await pool.query(
        'SELECT id, name, last_used, created_at FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC',
        [parent.id],
      );
      return rows;
    },
  },

  Mutation: {
    async upsertAlert(_: unknown, { input }: { input: { category: string; minGapScore: number; emailEnabled: boolean } }, ctx: GqlContext) {
      if (!ctx.userDbId) throw new Error('Unauthorized');
      const { rows } = await pool.query(
        `INSERT INTO alert_configs (user_id, category, min_gap_score, email_enabled)
         VALUES ($1, $2, $3, $4)
         ON CONFLICT (user_id, category) DO UPDATE
           SET min_gap_score = EXCLUDED.min_gap_score,
               email_enabled = EXCLUDED.email_enabled,
               updated_at = now()
         RETURNING *`,
        [ctx.userDbId, input.category, input.minGapScore, input.emailEnabled],
      );
      return mapAlert(rows[0]);
    },

    async deleteAlert(_: unknown, { category }: { category: string }, ctx: GqlContext) {
      if (!ctx.userDbId) throw new Error('Unauthorized');
      await pool.query(
        'DELETE FROM alert_configs WHERE user_id = $1 AND category = $2',
        [ctx.userDbId, category],
      );
      return true;
    },

    async createApiKey(_: unknown, { input }: { input: { name: string } }, ctx: GqlContext) {
      if (!ctx.userDbId) throw new Error('Unauthorized');
      const secret = `wb_${crypto.randomBytes(32).toString('hex')}`;
      const keyHash = crypto.createHash('sha256').update(secret).digest('hex');
      const { rows } = await pool.query(
        'INSERT INTO api_keys (user_id, key_hash, name) VALUES ($1, $2, $3) RETURNING id, name, created_at',
        [ctx.userDbId, keyHash, input.name],
      );
      return { ...rows[0], secret };
    },

    async revokeApiKey(_: unknown, { id }: { id: string }, ctx: GqlContext) {
      if (!ctx.userDbId) throw new Error('Unauthorized');
      await pool.query(
        'DELETE FROM api_keys WHERE id = $1 AND user_id = $2',
        [id, ctx.userDbId],
      );
      return true;
    },
  },
};

function mapUser(row: Record<string, unknown>) {
  return {
    id: row.id,
    auth0Id: row.auth0_id,
    email: row.email,
    name: row.name,
    plan: row.plan,
  };
}

function mapAlert(row: Record<string, unknown>) {
  return {
    id: row.id,
    category: row.category,
    minGapScore: row.min_gap_score,
    emailEnabled: row.email_enabled,
  };
}

function mapClusterBrief(row: Record<string, unknown>) {
  return {
    id: row.id,
    name: row.name,
    category: row.category,
    gapScore: row.gap_score,
    frequency: row.frequency,
    intensityAvg: row.intensity_avg,
    trendScore: row.trend_score,
    competitorGap: row.competitor_gap,
    postCount: row.post_count,
    createdAt: row.created_at,
    updatedAt: row.updated_at,
  };
}
