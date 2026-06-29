"""Unit tests for app.embedder -- Ollama embedding calls are mocked."""

import httpx
import pytest
import respx

from app import embedder
from app.config import settings


def test_build_embed_input_format():
    text = embedder.build_embed_input("Freelancers struggle with invoices", "An invoice tracker")
    assert text == "Freelancers struggle with invoices — An invoice tracker"


@pytest.mark.asyncio
async def test_generate_embedding_returns_768_dim_vector():
    fake_vector = [0.1] * 768

    with respx.mock(assert_all_called=True) as mock:
        mock.post(f"{settings.ollama_url}/api/embeddings").mock(
            return_value=httpx.Response(200, json={"embedding": fake_vector})
        )
        async with httpx.AsyncClient() as client:
            result = await embedder.generate_embedding(client, "some text")

    assert len(result) == 768
    assert result == fake_vector


@pytest.mark.asyncio
async def test_generate_embedding_wrong_dimension_raises():
    with respx.mock(assert_all_called=True) as mock:
        mock.post(f"{settings.ollama_url}/api/embeddings").mock(
            return_value=httpx.Response(200, json={"embedding": [0.1] * 10})
        )
        async with httpx.AsyncClient() as client:
            with pytest.raises(embedder.EmbeddingError):
                await embedder.generate_embedding(client, "some text")


@pytest.mark.asyncio
async def test_generate_embedding_http_error_raises_embedding_error():
    with respx.mock(assert_all_called=True) as mock:
        mock.post(f"{settings.ollama_url}/api/embeddings").mock(
            return_value=httpx.Response(500, json={"error": "internal error"})
        )
        async with httpx.AsyncClient() as client:
            with pytest.raises(embedder.EmbeddingError):
                await embedder.generate_embedding(client, "some text")
