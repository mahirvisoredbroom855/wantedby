"""FastAPI entrypoint for pain-classifier: exposes /health, /ready, /metrics and
launches the background worker pool on startup."""

import asyncio
from contextlib import asynccontextmanager

import httpx
import redis.asyncio as aioredis
import structlog
from fastapi import FastAPI
from motor.motor_asyncio import AsyncIOMotorClient
from prometheus_fastapi_instrumentator import Instrumentator

from app import qdrant
from app.config import settings
from app.worker import run_worker

logger = structlog.get_logger()


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Wires up all clients on startup, launches CLASSIFIER_WORKERS background
    workers, and tears everything down cleanly on shutdown."""
    redis_client = aioredis.from_url(settings.redis_url, decode_responses=True)
    mongo_client = AsyncIOMotorClient(settings.mongo_uri)
    qdrant_client = qdrant.build_client()
    http_client = httpx.AsyncClient()

    await qdrant.ensure_collection(qdrant_client)

    stop_event = asyncio.Event()
    worker_tasks = [
        asyncio.create_task(
            run_worker(i, redis_client, mongo_client, qdrant_client, http_client, stop_event)
        )
        for i in range(settings.classifier_workers)
    ]

    app.state.redis_client = redis_client
    app.state.mongo_client = mongo_client

    logger.info("classifier_started", workers=settings.classifier_workers)

    yield

    stop_event.set()
    await asyncio.gather(*worker_tasks, return_exceptions=True)
    await redis_client.aclose()
    mongo_client.close()
    await qdrant_client.close()
    await http_client.aclose()
    logger.info("classifier_stopped")


app = FastAPI(title="pain-classifier", lifespan=lifespan)
Instrumentator().instrument(app).expose(app, endpoint="/metrics")


@app.get("/health")
async def health() -> dict:
    """Liveness probe -- always returns ok if the process is running."""
    return {"status": "ok"}


@app.get("/ready")
async def ready() -> dict:
    """Readiness probe -- verifies Redis and MongoDB are reachable."""
    try:
        await app.state.redis_client.ping()
        await app.state.mongo_client.admin.command("ping")
    except Exception as exc:  # noqa: BLE001 -- any dependency failure means not ready
        return {"status": "not_ready", "error": str(exc)}
    return {"status": "ready"}
