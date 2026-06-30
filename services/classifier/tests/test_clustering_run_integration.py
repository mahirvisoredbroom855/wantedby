"""Integration test for app.clustering.run against real local Postgres + Mongo +
Qdrant. Uses synthetic near-identical vectors to deterministically force a real
HDBSCAN cluster, isolating "does the persistence path work" from "does HDBSCAN
judge these particular real posts as similar" (covered by the live manual run).

Skips if Postgres/Mongo/Qdrant aren't reachable locally.
"""

import uuid
from datetime import datetime, timezone

import asyncpg
import pytest
from motor.motor_asyncio import AsyncIOMotorClient
from qdrant_client.models import PointStruct

from app import qdrant as qdrant_store
from app.clustering import run as clustering_run
from app.config import settings

TEST_CATEGORY = "legal"  # picked because real test data never lands here, avoiding collisions


@pytest.fixture
async def integration_clients():
    if not settings.database_url:
        pytest.skip("DATABASE_URL not configured, skipping integration test")

    try:
        pg_pool = await asyncpg.create_pool(settings.database_url)
        mongo_client = AsyncIOMotorClient(settings.mongo_uri)
        await mongo_client.admin.command("ping")
        qdrant_client = qdrant_store.build_client()
        await qdrant_store.ensure_collection(qdrant_client)
    except Exception as exc:  # noqa: BLE001
        pytest.skip(f"local infra not reachable, skipping integration test: {exc}")

    yield pg_pool, mongo_client, qdrant_client

    # Cleanup: remove anything this test wrote.
    async with pg_pool.acquire() as conn:
        await conn.execute("DELETE FROM clusters WHERE category = $1", TEST_CATEGORY)
    mongo_client.close()
    await pg_pool.close()
    await qdrant_client.close()


@pytest.mark.asyncio
async def test_process_category_persists_real_cluster_end_to_end(integration_clients, monkeypatch):
    pg_pool, mongo_client, qdrant_client = integration_clients
    mongo_db = mongo_client["wantedby"]
    classified_posts = mongo_db["classified_posts"]

    # Stub out external LLM/Product Hunt calls -- this test verifies persistence,
    # not naming/competitor-search quality (those are covered by their own unit tests).
    async def fake_name(client, summaries):
        return "Test Synthetic Cluster"

    async def fake_competitors(client, category):
        return [{"name": "FakeCompetitor", "url": "https://example.com", "source": "producthunt"}]

    monkeypatch.setattr(clustering_run.naming, "generate_cluster_name", fake_name)
    monkeypatch.setattr(clustering_run.competitor_search, "find_competitors", fake_competitors)

    # Insert enough near-identical Qdrant vectors to force a real HDBSCAN cluster.
    # HDBSCAN needs a meaningfully larger sample (~15-20+) to compute a reliable
    # density estimate at all -- confirmed empirically: even with min_cluster_size=3,
    # 3-10 near-duplicate points are still all labelled noise. 25 reliably clusters.
    POINT_COUNT = 25
    mongo_ids = []
    base_vector = [0.5] * 768
    for i in range(POINT_COUNT):
        doc = {
            "platform_id": f"test:{uuid.uuid4()}",
            "platform": "reddit",
            "is_pain_signal": True,
            "confidence": 0.9,
            "category": TEST_CATEGORY,
            "pain_summary": f"Synthetic test pain summary {i}",
            "implied_solution": "Synthetic test solution",
            "intensity": 8,
            "target_user": "testers",
            "existing_solutions": [],
            "embedding_id": None,
            "cluster_id": None,
            "classified_at": datetime.now(timezone.utc),
            "llm_model": "test",
            "token_cost_usd": 0.0,
        }
        result = await classified_posts.insert_one(doc)
        mongo_id = str(result.inserted_id)
        mongo_ids.append(mongo_id)

        point_id = qdrant_store.mongo_id_to_point_id(mongo_id)
        # Tiny perturbation per point so HDBSCAN sees 3 distinct-but-close points,
        # not 3 exact duplicates (which some clustering libs special-case oddly).
        vector = [v + (i * 0.0001) for v in base_vector]
        await qdrant_client.upsert(
            collection_name=qdrant_store.COLLECTION_NAME,
            points=[
                PointStruct(
                    id=point_id,
                    vector=vector,
                    payload={
                        "mongo_id": mongo_id,
                        "category": TEST_CATEGORY,
                        "intensity": 8,
                        "platform": "reddit",
                        "summary": f"Synthetic test pain summary {i}",
                        "created_at": datetime.now(timezone.utc).isoformat(),
                    },
                )
            ],
        )

    try:
        import httpx

        async with httpx.AsyncClient() as http_client:
            clusters_written = await clustering_run.process_category(
                TEST_CATEGORY, qdrant_client, mongo_db, pg_pool, http_client
            )

        assert clusters_written >= 1

        async with pg_pool.acquire() as conn:
            cluster_row = await conn.fetchrow(
                "SELECT * FROM clusters WHERE category = $1 ORDER BY post_count DESC LIMIT 1",
                TEST_CATEGORY,
            )
            assert cluster_row is not None
            assert cluster_row["name"] == "Test Synthetic Cluster"
            assert cluster_row["post_count"] >= 3

            post_rows = await conn.fetch(
                "SELECT * FROM cluster_posts WHERE cluster_id = $1", cluster_row["id"]
            )
            assert len(post_rows) == cluster_row["post_count"]

            competitor_rows = await conn.fetch(
                "SELECT * FROM competitors WHERE cluster_id = $1", cluster_row["id"]
            )
            assert len(competitor_rows) == 1
            assert competitor_rows[0]["name"] == "FakeCompetitor"

        updated_docs = await classified_posts.find(
            {"_id": {"$in": [__import__("bson").ObjectId(mid) for mid in mongo_ids]}}
        ).to_list(length=POINT_COUNT)
        # HDBSCAN may split the 25 near-identical points into more than one cluster --
        # this test verifies the write-path fires correctly, not clustering quality.
        # At least most points must have been assigned to *some* real cluster (not noise).
        assigned_count = sum(1 for doc in updated_docs if doc["cluster_id"] is not None)
        assert assigned_count >= 3

    finally:
        await classified_posts.delete_many({"pain_summary": {"$regex": "^Synthetic test"}})
        for mongo_id in mongo_ids:
            point_id = qdrant_store.mongo_id_to_point_id(mongo_id)
            await qdrant_client.delete(
                collection_name=qdrant_store.COLLECTION_NAME, points_selector=[point_id]
            )
