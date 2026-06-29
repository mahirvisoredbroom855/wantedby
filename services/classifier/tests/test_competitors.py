"""Unit tests for app.clustering.competitors -- Product Hunt calls are mocked."""

import httpx
import pytest
import respx

from app.clustering import competitors
from app.config import settings


@pytest.mark.asyncio
async def test_find_competitors_returns_empty_when_token_missing(monkeypatch):
    monkeypatch.setattr(settings, "producthunt_api_token", "")
    async with httpx.AsyncClient() as client:
        result = await competitors.find_competitors(client, "finance")
    assert result == []


@pytest.mark.asyncio
async def test_find_competitors_returns_empty_for_unmapped_category(monkeypatch):
    monkeypatch.setattr(settings, "producthunt_api_token", "fake-token")
    async with httpx.AsyncClient() as client:
        result = await competitors.find_competitors(client, "other")
    assert result == []


@pytest.mark.asyncio
async def test_find_competitors_happy_path(monkeypatch):
    monkeypatch.setattr(settings, "producthunt_api_token", "fake-token")

    with respx.mock(assert_all_called=True) as mock:
        mock.post(competitors.PRODUCTHUNT_GRAPHQL_URL).mock(
            side_effect=[
                httpx.Response(
                    200,
                    json={"data": {"topics": {"edges": [{"node": {"slug": "fintech", "name": "Fintech"}}]}}},
                ),
                httpx.Response(
                    200,
                    json={
                        "data": {
                            "posts": {
                                "edges": [
                                    {"node": {"name": "CompetitorOne", "url": "https://producthunt.com/x"}},
                                ]
                            }
                        }
                    },
                ),
            ]
        )
        async with httpx.AsyncClient() as client:
            result = await competitors.find_competitors(client, "finance")

    assert result == [{"name": "CompetitorOne", "url": "https://producthunt.com/x", "source": "producthunt"}]


@pytest.mark.asyncio
async def test_find_competitors_returns_empty_when_no_matching_topic(monkeypatch):
    monkeypatch.setattr(settings, "producthunt_api_token", "fake-token")

    with respx.mock(assert_all_called=True) as mock:
        mock.post(competitors.PRODUCTHUNT_GRAPHQL_URL).mock(
            return_value=httpx.Response(200, json={"data": {"topics": {"edges": []}}})
        )
        async with httpx.AsyncClient() as client:
            result = await competitors.find_competitors(client, "finance")

    assert result == []


@pytest.mark.asyncio
async def test_find_competitors_returns_empty_on_graphql_error(monkeypatch):
    monkeypatch.setattr(settings, "producthunt_api_token", "fake-token")

    with respx.mock(assert_all_called=True) as mock:
        mock.post(competitors.PRODUCTHUNT_GRAPHQL_URL).mock(
            return_value=httpx.Response(200, json={"errors": [{"message": "boom"}]})
        )
        async with httpx.AsyncClient() as client:
            result = await competitors.find_competitors(client, "finance")

    assert result == []
