"""Embedding generation via nomic-embed-text through Ollama."""

import httpx
import structlog

from app.config import settings

logger = structlog.get_logger()


class EmbeddingError(Exception):
    """Raised when the embedding call fails or returns malformed output."""


def build_embed_input(pain_summary: str, implied_solution: str) -> str:
    """Builds the text to embed: pain_summary + ' — ' + implied_solution, per spec."""
    return f"{pain_summary} — {implied_solution}"


async def generate_embedding(client: httpx.AsyncClient, text: str) -> list[float]:
    """Generates a 768-dim embedding vector for the given text via Ollama's
    nomic-embed-text model. Raises EmbeddingError on failure.
    """
    try:
        response = await client.post(
            f"{settings.ollama_url}/api/embeddings",
            json={"model": settings.embedding_model, "prompt": text},
            timeout=30.0,
        )
        response.raise_for_status()
        data = response.json()
        embedding = data["embedding"]
    except (httpx.HTTPError, KeyError) as exc:
        raise EmbeddingError(f"embedding generation failed: {exc}") from exc

    if not isinstance(embedding, list) or len(embedding) != 768:
        raise EmbeddingError(f"unexpected embedding shape: len={len(embedding) if isinstance(embedding, list) else 'n/a'}")

    return embedding
