"""Auto-generates a short, human-readable cluster name from its member posts' summaries."""

import json

import httpx

from app.config import settings

NAMING_SYSTEM_PROMPT = (
    "You are a product analyst naming a cluster of related product pain points. "
    "Given several one-sentence pain summaries that all describe the same underlying "
    "problem, produce a short, punchy cluster name (4-8 words) that captures the "
    "common theme. Respond ONLY with valid JSON: {\"name\": \"...\"}. No preamble."
)

MAX_SUMMARIES_IN_PROMPT = 10


async def generate_cluster_name(client: httpx.AsyncClient, pain_summaries: list[str]) -> str:
    """Generates a cluster name from a sample of its member posts' pain summaries.
    Falls back to a truncated version of the first summary if the LLM call fails,
    since a missing name should never block the clustering run.
    """
    sample = pain_summaries[:MAX_SUMMARIES_IN_PROMPT]
    user_prompt = "Pain summaries:\n" + "\n".join(f"- {s}" for s in sample)

    try:
        response = await client.post(
            f"{settings.ollama_url}/api/generate",
            json={
                "model": settings.llm_model_dev,
                "system": NAMING_SYSTEM_PROMPT,
                "prompt": user_prompt,
                "stream": False,
                "format": "json",
                "options": {"temperature": 0.3},
            },
            timeout=180.0,
        )
        response.raise_for_status()
        data = json.loads(response.json()["response"])
        name = data["name"].strip()
        if name:
            return name
    except (httpx.HTTPError, json.JSONDecodeError, KeyError):
        pass

    fallback = pain_summaries[0] if pain_summaries else "Untitled cluster"
    return fallback[:60]
