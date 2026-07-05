"""Nightly clustering job: fetches Qdrant vectors by category, runs HDBSCAN,
writes cluster assignments to MongoDB, and upserts clusters + gap scores to
PostgreSQL. Designed to run as a standalone script (Kubernetes CronJob in prod).
"""

import asyncio
from datetime import datetime, timezone

import asyncpg
import hdbscan
import httpx
import numpy as np
import structlog
from motor.motor_asyncio import AsyncIOMotorClient
from qdrant_client import AsyncQdrantClient
from qdrant_client.models import FieldCondition, Filter, MatchValue

from app import qdrant as qdrant_store
from app.clustering import competitors as competitor_search
from app.clustering import naming
from app.clustering.gap_score import compute_gap_score
from app.config import settings
from app.models import PainCategory

logger = structlog.get_logger()

MIN_CLUSTER_SIZE = 3  # HDBSCAN: smallest group of points considered a real cluster
SCROLL_BATCH_SIZE = 256


async def scroll_category_vectors(
    client: AsyncQdrantClient, category: str
) -> tuple[list[str], list[list[float]], list[dict]]:
    """Fetches all Qdrant points for one category via the scroll API.
    Returns parallel lists of point IDs, vectors, and payloads.
    """
    point_ids: list[str] = []
    vectors: list[list[float]] = []
    payloads: list[dict] = []

    offset = None
    while True:
        result, offset = await client.scroll(
            collection_name=qdrant_store.COLLECTION_NAME,
            scroll_filter=Filter(
                must=[FieldCondition(key="category", match=MatchValue(value=category))]
            ),
            limit=SCROLL_BATCH_SIZE,
            offset=offset,
            with_vectors=True,
        )
        for point in result:
            point_ids.append(str(point.id))
            vectors.append(point.vector)
            payloads.append(point.payload)

        if offset is None:
            break

    return point_ids, vectors, payloads


def run_hdbscan(vectors: list[list[float]]) -> np.ndarray:
    """Runs HDBSCAN over the given vectors. Returns a label array where -1 means
    noise (no cluster assigned), per the spec.
    """
    matrix = np.array(vectors)
    clusterer = hdbscan.HDBSCAN(min_cluster_size=MIN_CLUSTER_SIZE, metric="euclidean")
    return clusterer.fit_predict(matrix)


async def process_category(
    category: str,
    qdrant_client: AsyncQdrantClient,
    mongo_db,
    pg_pool: asyncpg.Pool,
    http_client: httpx.AsyncClient,
) -> int:
    """Runs the full clustering pipeline for one category. Returns the number of
    clusters written.
    """
    log = logger.bind(category=category)

    point_ids, vectors, payloads = await scroll_category_vectors(qdrant_client, category)
    if len(vectors) < MIN_CLUSTER_SIZE:
        log.info("skipping_category_insufficient_points", point_count=len(vectors))
        return 0

    labels = run_hdbscan(vectors)
    log.info("hdbscan_complete", point_count=len(vectors), cluster_count=len(set(labels)) - (1 if -1 in labels else 0))

    classified_posts = mongo_db["classified_posts"]
    clusters_written = 0

    unique_labels = sorted(set(labels))
    for label in unique_labels:
        if label == -1:
            continue  # noise, per spec -- left unassigned (cluster_id stays null)

        member_indices = [i for i, lbl in enumerate(labels) if lbl == label]
        member_payloads = [payloads[i] for i in member_indices]
        member_mongo_ids = [p["mongo_id"] for p in member_payloads]
        member_summaries = [p["summary"] for p in member_payloads]
        member_intensities = [p["intensity"] for p in member_payloads]

        intensity_avg = sum(member_intensities) / len(member_intensities)
        post_count = len(member_indices)

        cluster_name = await naming.generate_cluster_name(http_client, member_summaries)
        competitor_list = await competitor_search.find_competitors(http_client, category)

        gap_score = compute_gap_score(
            post_count=post_count,
            intensity_avg=intensity_avg,
            current_period_count=post_count,
            previous_period_count=None,  # first-run baseline; trend tracked on subsequent runs
            competitor_count=len(competitor_list),
        )

        now = datetime.now(timezone.utc)
        cluster_id = await _upsert_cluster(
            pg_pool,
            name=cluster_name,
            category=category,
            gap_score=gap_score,
            post_count=post_count,
            intensity_avg=intensity_avg,
            representative_quote=member_summaries[0],
            now=now,
        )

        await _upsert_cluster_posts(pg_pool, cluster_id, member_payloads)
        await _upsert_competitors(pg_pool, cluster_id, competitor_list)
        await _update_mongo_cluster_ids(classified_posts, member_mongo_ids, str(cluster_id))

        clusters_written += 1
        log.info("cluster_written", cluster_id=str(cluster_id), post_count=post_count, gap_score=round(gap_score, 2))

    return clusters_written


async def _upsert_cluster(
    pg_pool: asyncpg.Pool,
    name: str,
    category: str,
    gap_score: float,
    post_count: int,
    intensity_avg: float,
    representative_quote: str,
    now: datetime,
):
    async with pg_pool.acquire() as conn:
        row = await conn.fetchrow(
            """
            INSERT INTO clusters
                (name, category, gap_score, frequency, intensity_avg,
                 representative_quote, first_seen, last_seen, post_count, updated_at)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $7, $4, $7)
            RETURNING id
            """,
            name, category, gap_score, post_count, intensity_avg, representative_quote, now,
        )
        return row["id"]


async def _upsert_cluster_posts(pg_pool: asyncpg.Pool, cluster_id, member_payloads: list[dict]):
    async with pg_pool.acquire() as conn:
        for payload in member_payloads:
            await conn.execute(
                """
                INSERT INTO cluster_posts (cluster_id, mongo_classified_id, platform, pain_summary, intensity)
                VALUES ($1, $2, $3, $4, $5)
                ON CONFLICT (mongo_classified_id) DO UPDATE SET cluster_id = EXCLUDED.cluster_id
                """,
                cluster_id, payload["mongo_id"], payload["platform"], payload["summary"], payload["intensity"],
            )


async def _upsert_competitors(pg_pool: asyncpg.Pool, cluster_id, competitor_list: list[dict]):
    if not competitor_list:
        return
    async with pg_pool.acquire() as conn:
        for comp in competitor_list:
            await conn.execute(
                "INSERT INTO competitors (cluster_id, name, url, source) VALUES ($1, $2, $3, $4)",
                cluster_id, comp["name"], comp["url"], comp["source"],
            )


async def _update_mongo_cluster_ids(classified_posts, mongo_ids: list[str], cluster_id: str):
    from bson import ObjectId

    await classified_posts.update_many(
        {"_id": {"$in": [ObjectId(mid) for mid in mongo_ids]}},
        {"$set": {"cluster_id": cluster_id}},
    )


async def run_clustering() -> None:
    """Entry point: runs clustering for every category, sequentially."""
    qdrant_client = qdrant_store.build_client()
    mongo_client = AsyncIOMotorClient(settings.mongo_uri)
    mongo_db = mongo_client["wantedby"]
    pg_pool = await asyncpg.create_pool(settings.database_url)
    http_client = httpx.AsyncClient()

    total_clusters = 0
    try:
        for category in PainCategory:
            count = await process_category(category.value, qdrant_client, mongo_db, pg_pool, http_client)
            total_clusters += count
    finally:
        await qdrant_client.close()
        mongo_client.close()
        await pg_pool.close()
        await http_client.aclose()

    logger.info("clustering_run_complete", total_clusters=total_clusters)


if __name__ == "__main__":
    structlog.configure(processors=[structlog.processors.JSONRenderer()])
    asyncio.run(run_clustering())
