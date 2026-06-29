"""Background worker loop: consumes job IDs from Redis, classifies, embeds, persists."""

import asyncio
import json
from datetime import datetime, timezone

import httpx
import redis.asyncio as aioredis
import structlog
from bson import ObjectId
from motor.motor_asyncio import AsyncIOMotorClient
from qdrant_client import AsyncQdrantClient

from app import classifier, embedder, qdrant
from app.config import settings

logger = structlog.get_logger()

QUEUE_NAME = "queue:classify"
BRPOP_TIMEOUT_SECONDS = 5


async def run_worker(
    worker_id: int,
    redis_client: aioredis.Redis,
    mongo_client: AsyncIOMotorClient,
    qdrant_client: AsyncQdrantClient,
    http_client: httpx.AsyncClient,
    stop_event: asyncio.Event,
) -> None:
    """Runs a single worker's consume loop until stop_event is set."""
    db = mongo_client["wantedby"]
    raw_posts = db["raw_posts"]
    classified_posts = db["classified_posts"]

    log = logger.bind(worker_id=worker_id)
    log.info("worker_started")

    while not stop_event.is_set():
        try:
            result = await redis_client.brpop([QUEUE_NAME], timeout=BRPOP_TIMEOUT_SECONDS)
        except Exception as exc:  # noqa: BLE001 -- transient Redis errors must not kill the worker
            log.error("brpop_failed", error=str(exc))
            await asyncio.sleep(1)
            continue

        if result is None:
            continue  # timeout, no job -- loop and check stop_event again

        _, raw_job = result
        try:
            job = json.loads(raw_job)
            post_id = job["post_id"]
        except (json.JSONDecodeError, KeyError) as exc:
            log.error("invalid_job_payload", raw_job=raw_job, error=str(exc))
            continue

        await _process_job(log, post_id, raw_posts, classified_posts, qdrant_client, http_client)

    log.info("worker_stopped")


async def _process_job(
    log: structlog.BoundLogger,
    post_id: str,
    raw_posts,
    classified_posts,
    qdrant_client: AsyncQdrantClient,
    http_client: httpx.AsyncClient,
) -> None:
    """Processes a single job end-to-end: fetch, classify, embed, persist."""
    job_log = log.bind(post_id=post_id)

    raw_post = await raw_posts.find_one({"_id": ObjectId(post_id)})
    if raw_post is None:
        job_log.error("raw_post_not_found")
        return

    try:
        classification = await classifier.classify_post(
            http_client,
            platform=raw_post["platform"],
            subreddit=raw_post.get("subreddit", ""),
            title=raw_post["title"],
            body=raw_post["body"],
        )
    except classifier.ClassificationError as exc:
        job_log.error("classification_failed", error=str(exc))
        await raw_posts.update_one({"_id": ObjectId(post_id)}, {"$set": {"status": "failed"}})
        return

    embedding_id = None
    if classification.is_pain_signal:
        embed_input = embedder.build_embed_input(
            classification.pain_summary, classification.implied_solution
        )
        try:
            vector = await embedder.generate_embedding(http_client, embed_input)
            embedding_id = await qdrant.upsert_point(
                qdrant_client,
                mongo_id=post_id,
                vector=vector,
                category=classification.category.value,
                intensity=classification.intensity,
                platform=raw_post["platform"],
                summary=classification.pain_summary,
                created_at=datetime.now(timezone.utc),
            )
        except embedder.EmbeddingError as exc:
            job_log.error("embedding_failed", error=str(exc))
            # Classification still succeeded -- persist it without an embedding
            # rather than discarding a valid LLM result over a downstream failure.

    classified_doc = {
        "raw_post_id": ObjectId(post_id),
        "platform_id": raw_post["platform_id"],
        "platform": raw_post["platform"],
        "is_pain_signal": classification.is_pain_signal,
        "confidence": classification.confidence,
        "category": classification.category.value,
        "pain_summary": classification.pain_summary,
        "implied_solution": classification.implied_solution,
        "intensity": classification.intensity,
        "target_user": classification.target_user,
        "existing_solutions": classification.existing_solutions_mentioned,
        "embedding_id": embedding_id,
        "cluster_id": None,
        "classified_at": datetime.now(timezone.utc),
        "llm_model": settings.llm_model_dev,
        "token_cost_usd": 0.0,
    }
    await classified_posts.insert_one(classified_doc)
    await raw_posts.update_one({"_id": ObjectId(post_id)}, {"$set": {"status": "classified"}})

    job_log.info(
        "post_classified",
        is_pain_signal=classification.is_pain_signal,
        category=classification.category.value,
    )
