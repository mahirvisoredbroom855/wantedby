"""Qdrant vector store operations for the pain_signals collection."""

import uuid
from datetime import datetime

from qdrant_client import AsyncQdrantClient
from qdrant_client.models import Distance, PointStruct, VectorParams

from app.config import settings

COLLECTION_NAME = "pain_signals"
VECTOR_SIZE = 768


def mongo_id_to_point_id(mongo_id: str) -> str:
    """Derives a deterministic UUID from a MongoDB ObjectId hex string.

    Qdrant point IDs must be an unsigned integer or a valid UUID -- a raw
    24-char ObjectId hex string satisfies neither constraint. uuid5 gives a
    stable, collision-resistant mapping; the original mongo_id is still kept
    in the point payload for reverse lookup.
    """
    return str(uuid.uuid5(uuid.NAMESPACE_OID, mongo_id))


def build_client() -> AsyncQdrantClient:
    """Constructs the async Qdrant client from configured settings."""
    return AsyncQdrantClient(url=settings.qdrant_url, api_key=settings.qdrant_api_key or None)


async def ensure_collection(client: AsyncQdrantClient) -> None:
    """Creates the pain_signals collection if it doesn't already exist."""
    collections = await client.get_collections()
    existing_names = {c.name for c in collections.collections}
    if COLLECTION_NAME not in existing_names:
        await client.create_collection(
            collection_name=COLLECTION_NAME,
            vectors_config=VectorParams(size=VECTOR_SIZE, distance=Distance.COSINE),
        )


async def upsert_point(
    client: AsyncQdrantClient,
    mongo_id: str,
    vector: list[float],
    category: str,
    intensity: int,
    platform: str,
    summary: str,
    created_at: datetime,
) -> str:
    """Upserts one embedding vector with its payload, per the spec's exact schema.
    Returns the derived Qdrant point ID (a UUID) for storage as embedding_id.
    """
    point_id = mongo_id_to_point_id(mongo_id)
    await client.upsert(
        collection_name=COLLECTION_NAME,
        points=[
            PointStruct(
                id=point_id,
                vector=vector,
                payload={
                    "mongo_id": mongo_id,
                    "category": category,
                    "intensity": intensity,
                    "platform": platform,
                    "summary": summary,
                    "created_at": created_at.isoformat(),
                },
            )
        ],
    )
    return point_id
