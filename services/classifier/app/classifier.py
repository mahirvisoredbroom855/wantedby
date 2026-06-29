"""LLM-based pain signal classification via Ollama (dev) or Anthropic Claude (prod)."""

import asyncio
import json

import httpx
import structlog

from app.config import settings
from app.models import ClassificationResult

logger = structlog.get_logger()

SYSTEM_PROMPT = (
    "You are a product intelligence analyst. Your job is to analyse social media "
    "posts and identify genuine product pain signals — moments where someone "
    "expresses frustration with a missing, broken, or inadequate tool or solution. "
    "Respond ONLY with valid JSON. No preamble."
)

_RESPONSE_SCHEMA_HINT = """Return this exact JSON structure:
{
  "is_pain_signal": true | false,
  "confidence": 0.0-1.0,
  "category": "devtools" | "productivity" | "finance" | "health" | "education" | "marketing" | "hr" | "legal" | "other",
  "pain_summary": "one sentence: what exactly is the person struggling with",
  "implied_solution": "one sentence: what product would solve this",
  "intensity": 1-10,
  "target_user": "who experiences this pain",
  "existing_solutions_mentioned": ["array of any tools they mentioned"],
  "reasoning": "brief explanation of your classification"
}"""

MAX_RETRIES = 3
BASE_BACKOFF_SECONDS = 1.0


def build_user_prompt(platform: str, subreddit: str, title: str, body: str) -> str:
    """Builds the user-turn prompt for one post, per the spec's exact structure."""
    return (
        f"Analyse this post and return JSON:\n\n"
        f"PLATFORM: {platform}\n"
        f"SUBREDDIT: {subreddit}\n"
        f"TITLE: {title}\n"
        f"BODY: {body}\n\n"
        f"{_RESPONSE_SCHEMA_HINT}"
    )


class ClassificationError(Exception):
    """Raised when the LLM call fails or returns an unparseable/invalid response."""


async def classify_post(
    client: httpx.AsyncClient,
    platform: str,
    subreddit: str,
    title: str,
    body: str,
) -> ClassificationResult:
    """Classifies one post via the configured LLM, retrying up to MAX_RETRIES times
    with exponential backoff on failure. Raises ClassificationError if all retries fail.
    """
    user_prompt = build_user_prompt(platform, subreddit, title, body)

    last_error: Exception | None = None
    for attempt in range(1, MAX_RETRIES + 1):
        try:
            raw_response = await _call_ollama(client, user_prompt)
            return _parse_response(raw_response)
        except (httpx.HTTPError, json.JSONDecodeError, ValueError) as exc:
            last_error = exc
            logger.warning(
                "classification_attempt_failed",
                attempt=attempt,
                max_retries=MAX_RETRIES,
                error=str(exc),
            )
            if attempt < MAX_RETRIES:
                await asyncio.sleep(BASE_BACKOFF_SECONDS * (2 ** (attempt - 1)))

    raise ClassificationError(f"classification failed after {MAX_RETRIES} attempts: {last_error}")


async def _call_ollama(client: httpx.AsyncClient, user_prompt: str) -> str:
    """Calls Ollama's /api/generate endpoint with the system+user prompt, temperature 0.1."""
    response = await client.post(
        f"{settings.ollama_url}/api/generate",
        json={
            "model": settings.llm_model_dev,
            "system": SYSTEM_PROMPT,
            "prompt": user_prompt,
            "stream": False,
            "format": "json",
            "options": {"temperature": 0.1},
        },
        # CPU-only inference for a 3B model is genuinely slow (observed 17s for a
        # 3-token reply on this hardware) -- 60s was too aggressive and triggered
        # spurious retries before generation could finish. 180s reflects real
        # local-dev latency, not a workaround for a bug.
        timeout=180.0,
    )
    response.raise_for_status()
    data = response.json()
    return data["response"]


def _parse_response(raw_response: str) -> ClassificationResult:
    """Parses and validates the LLM's raw JSON text into a ClassificationResult."""
    payload = json.loads(raw_response)
    return ClassificationResult.model_validate(payload)
