import { pool } from '../../db/postgres';
import type { AuthUser } from '../../auth/middleware';

export interface ClusterFilters {
  category?: string;
  minGapScore?: number;
  limit?: number;
  offset?: number;
}

export interface GqlContext {
  user?: AuthUser;
  userDbId?: string;
}

export const clusterResolvers = {
  Query: {
    async clusters(_: unknown, { filters = {} }: { filters?: ClusterFilters }, ctx: GqlContext) {
      const { category, minGapScore, limit = 20, offset = 0 } = filters;
      const conditions: string[] = [];
      const params: unknown[] = [];

      if (category) {
        params.push(category);
        conditions.push(`category = $${params.length}`);
      }
      if (minGapScore !== undefined) {
        params.push(minGapScore);
        conditions.push(`gap_score >= $${params.length}`);
      }

      const where = conditions.length ? `WHERE ${conditions.join(' AND ')}` : '';
      params.push(limit, offset);

      const rows = await pool.query(
        `SELECT * FROM clusters ${where} ORDER BY gap_score DESC LIMIT $${params.length - 1} OFFSET $${params.length}`,
        params,
      );
      return rows.rows.map((r) => mapCluster(r, ctx));
    },

    async cluster(_: unknown, { id }: { id: string }, ctx: GqlContext) {
      const { rows } = await pool.query('SELECT * FROM clusters WHERE id = $1', [id]);
      return rows[0] ? mapCluster(rows[0], ctx) : null;
    },

    async trending(_: unknown, { limit = 10 }: { limit?: number }, ctx: GqlContext) {
      const { rows } = await pool.query(
        `SELECT * FROM clusters WHERE created_at > now() - interval '7 days'
         ORDER BY trend_score DESC, gap_score DESC LIMIT $1`,
        [limit],
      );
      return rows.map((r) => mapCluster(r, ctx));
    },
  },

  Cluster: {
    async posts(parent: { id: string }) {
      const { rows } = await pool.query(
        `SELECT cp.*, c.title, c.url, c.score
         FROM cluster_posts cp
         LEFT JOIN classified_posts_view c ON cp.mongo_classified_id = c.mongo_id
         WHERE cp.cluster_id = $1`,
        [parent.id],
      );
      return rows.map(mapPost);
    },

    async competitors(parent: { id: string }) {
      const { rows } = await pool.query(
        'SELECT * FROM competitors WHERE cluster_id = $1',
        [parent.id],
      );
      return rows;
    },

    async isSaved(parent: { id: string }, _: unknown, ctx: GqlContext) {
      if (!ctx.userDbId) return false;
      const { rows } = await pool.query(
        'SELECT 1 FROM saved_clusters WHERE user_id = $1 AND cluster_id = $2',
        [ctx.userDbId, parent.id],
      );
      return rows.length > 0;
    },
  },

  Mutation: {
    async saveCluster(_: unknown, { input }: { input: { clusterId: string } }, ctx: GqlContext) {
      if (!ctx.userDbId) throw new Error('Unauthorized');
      await pool.query(
        'INSERT INTO saved_clusters (user_id, cluster_id) VALUES ($1, $2) ON CONFLICT DO NOTHING',
        [ctx.userDbId, input.clusterId],
      );
      const { rows } = await pool.query('SELECT * FROM clusters WHERE id = $1', [input.clusterId]);
      return mapCluster(rows[0], ctx);
    },

    async unsaveCluster(_: unknown, { clusterId }: { clusterId: string }, ctx: GqlContext) {
      if (!ctx.userDbId) throw new Error('Unauthorized');
      await pool.query(
        'DELETE FROM saved_clusters WHERE user_id = $1 AND cluster_id = $2',
        [ctx.userDbId, clusterId],
      );
      return true;
    },
  },
};

function mapCluster(row: Record<string, unknown>, _ctx: GqlContext) {
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

function mapPost(row: Record<string, unknown>) {
  return {
    id: row.id,
    mongoClassifiedId: row.mongo_classified_id,
    platform: row.platform,
    platformId: row.platform_id,
    title: row.title ?? '',
    body: row.body,
    url: row.url,
    score: row.score ?? 0,
    painSummary: row.pain_summary,
    intensity: row.intensity,
    createdAt: row.classified_at ?? row.created_at,
  };
}
