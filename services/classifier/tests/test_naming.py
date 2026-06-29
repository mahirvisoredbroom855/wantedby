"""Unit tests for app.clustering.naming -- Ollama calls are mocked."""

import json

import httpx
import pytest
import respx

from app.clustering import naming
from app.config import settings


@pytest.mark.asyncio
async def test_generate_cluster_name_happy_path():
    with respx.mock(assert_all_called=True) as mock:
        mock.post(f"{settings.ollama_url}/api/generate").mock(
            return_value=httpx.Response(
                200, json={"response": json.dumps({"name": "Freelancer Invoice Tracking Gap"})}
            )
        )
        async with httpx.AsyncClient() as client:
            name = await naming.generate_cluster_name(client, ["Freelancers struggle to track invoices"])

    assert name == "Freelancer Invoice Tracking Gap"


@pytest.mark.asyncio
async def test_generate_cluster_name_falls_back_on_http_error():
    with respx.mock(assert_all_called=True) as mock:
        mock.post(f"{settings.ollama_url}/api/generate").mock(return_value=httpx.Response(500))
        async with httpx.AsyncClient() as client:
            name = await naming.generate_cluster_name(client, ["Some pain summary here"])

    assert name == "Some pain summary here"


@pytest.mark.asyncio
async def test_generate_cluster_name_falls_back_on_empty_name():
    with respx.mock(assert_all_called=True) as mock:
        mock.post(f"{settings.ollama_url}/api/generate").mock(
            return_value=httpx.Response(200, json={"response": json.dumps({"name": "  "})})
        )
        async with httpx.AsyncClient() as client:
            name = await naming.generate_cluster_name(client, ["Fallback summary text"])

    assert name == "Fallback summary text"


@pytest.mark.asyncio
async def test_generate_cluster_name_handles_no_summaries():
    with respx.mock(assert_all_called=True) as mock:
        mock.post(f"{settings.ollama_url}/api/generate").mock(return_value=httpx.Response(500))
        async with httpx.AsyncClient() as client:
            name = await naming.generate_cluster_name(client, [])

    assert name == "Untitled cluster"
