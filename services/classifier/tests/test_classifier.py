"""Unit tests for app.classifier -- LLM calls are mocked with fixture responses,
never hitting a real Ollama/Claude endpoint."""

import json

import httpx
import pytest

from app import classifier
from app.models import ClassificationResult

VALID_FIXTURE = {
    "is_pain_signal": True,
    "confidence": 0.92,
    "category": "productivity",
    "pain_summary": "Freelancers struggle to track invoice payment status",
    "implied_solution": "A simple invoice tracker with payment status",
    "intensity": 7,
    "target_user": "freelancers",
    "existing_solutions_mentioned": ["spreadsheets"],
    "reasoning": "Clear frustration with manual invoice tracking",
}


def test_build_user_prompt_contains_all_fields():
    prompt = classifier.build_user_prompt("reddit", "freelance", "My title", "My body")
    assert "PLATFORM: reddit" in prompt
    assert "SUBREDDIT: freelance" in prompt
    assert "TITLE: My title" in prompt
    assert "BODY: My body" in prompt


def test_parse_response_valid_json_returns_classification_result():
    result = classifier._parse_response(json.dumps(VALID_FIXTURE))
    assert isinstance(result, ClassificationResult)
    assert result.is_pain_signal is True
    assert result.category.value == "productivity"
    assert result.intensity == 7


def test_parse_response_invalid_json_raises():
    with pytest.raises(json.JSONDecodeError):
        classifier._parse_response("not valid json{{{")


def test_parse_response_missing_required_field_raises():
    broken = dict(VALID_FIXTURE)
    del broken["confidence"]
    with pytest.raises(Exception):  # pydantic.ValidationError
        classifier._parse_response(json.dumps(broken))


def test_parse_response_intensity_out_of_range_raises():
    broken = dict(VALID_FIXTURE)
    broken["intensity"] = 15
    with pytest.raises(Exception):  # pydantic.ValidationError
        classifier._parse_response(json.dumps(broken))


@pytest.mark.asyncio
async def test_classify_post_succeeds_on_first_attempt(monkeypatch):
    async def fake_call_ollama(client, prompt):
        return json.dumps(VALID_FIXTURE)

    monkeypatch.setattr(classifier, "_call_ollama", fake_call_ollama)

    async with httpx.AsyncClient() as client:
        result = await classifier.classify_post(client, "reddit", "freelance", "title", "body")

    assert result.is_pain_signal is True
    assert result.pain_summary == VALID_FIXTURE["pain_summary"]


@pytest.mark.asyncio
async def test_classify_post_retries_then_succeeds(monkeypatch):
    attempts = {"count": 0}

    async def flaky_call_ollama(client, prompt):
        attempts["count"] += 1
        if attempts["count"] < 2:
            raise httpx.ConnectError("connection refused")
        return json.dumps(VALID_FIXTURE)

    monkeypatch.setattr(classifier, "_call_ollama", flaky_call_ollama)
    monkeypatch.setattr(classifier, "BASE_BACKOFF_SECONDS", 0.01)

    async with httpx.AsyncClient() as client:
        result = await classifier.classify_post(client, "reddit", "freelance", "title", "body")

    assert attempts["count"] == 2
    assert result.is_pain_signal is True


@pytest.mark.asyncio
async def test_classify_post_raises_after_max_retries(monkeypatch):
    async def always_fails(client, prompt):
        raise httpx.ConnectError("connection refused")

    monkeypatch.setattr(classifier, "_call_ollama", always_fails)
    monkeypatch.setattr(classifier, "BASE_BACKOFF_SECONDS", 0.01)

    async with httpx.AsyncClient() as client:
        with pytest.raises(classifier.ClassificationError):
            await classifier.classify_post(client, "reddit", "freelance", "title", "body")
