"""Competitor detection via Product Hunt's topic-based posts API.

Product Hunt's official API has no free-text search over posts (confirmed via
schema introspection -- `posts` only filters by topic/date/featured, and `topics`
search matches topic names, not product descriptions). This is a real platform
constraint, not a missing-credentials problem.

Approximation used here: map a cluster's category to a Product Hunt topic by
searching topic names, then treat the top-voted posts in that topic as loose
competitor signals. This is imprecise -- it tells you "this category is crowded
on PH," not "this exact pain point already has a solution" -- but it's the most
honest signal available through the legitimate API.
"""

import httpx
import structlog

from app.config import settings

logger = structlog.get_logger()

PRODUCTHUNT_GRAPHQL_URL = "https://api.producthunt.com/v2/api/graphql"

# Maps our internal category slugs to Product Hunt topic search terms. PH topic
# names don't always match our category vocabulary 1:1 (e.g. "hr" -> "Human Resources").
CATEGORY_TO_TOPIC_QUERY = {
    "devtools": "Developer Tools",
    "productivity": "Productivity",
    "finance": "Fintech",
    "health": "Health",
    "education": "Education",
    "marketing": "Marketing",
    "hr": "Human Resources",
    "legal": "Legal",
    "other": "",
}

_FIND_TOPIC_QUERY = """
query FindTopic($query: String!) {
  topics(first: 1, query: $query) {
    edges {
      node { slug name }
    }
  }
}
"""

_POSTS_BY_TOPIC_QUERY = """
query PostsByTopic($topic: String!) {
  posts(first: 5, topic: $topic, order: VOTES) {
    edges {
      node { name url }
    }
  }
}
"""


class CompetitorSearchError(Exception):
    """Raised when a Product Hunt API call fails or returns GraphQL errors."""


async def _graphql(client: httpx.AsyncClient, query: str, variables: dict) -> dict:
    response = await client.post(
        PRODUCTHUNT_GRAPHQL_URL,
        headers={"Authorization": f"Bearer {settings.producthunt_api_token}"},
        json={"query": query, "variables": variables},
        timeout=30.0,
    )
    response.raise_for_status()
    data = response.json()
    if "errors" in data:
        raise CompetitorSearchError(str(data["errors"]))
    return data["data"]


async def find_competitors(client: httpx.AsyncClient, category: str) -> list[dict]:
    """Finds loose competitor signals for a cluster's category via Product Hunt's
    topic-based posts API. Returns a list of {"name", "url", "source"} dicts.
    Returns an empty list (not an error) on any failure or missing config --
    a competitor search failure should reduce gap-score accuracy, not abort the
    clustering run.
    """
    if not settings.producthunt_api_token:
        logger.warning("producthunt_token_missing", action="skipping_competitor_search")
        return []

    topic_query = CATEGORY_TO_TOPIC_QUERY.get(category, "")
    if not topic_query:
        return []

    try:
        topic_data = await _graphql(client, _FIND_TOPIC_QUERY, {"query": topic_query})
        topic_edges = topic_data["topics"]["edges"]
        if not topic_edges:
            return []
        topic_slug = topic_edges[0]["node"]["slug"]

        posts_data = await _graphql(client, _POSTS_BY_TOPIC_QUERY, {"topic": topic_slug})
        post_edges = posts_data["posts"]["edges"]
        return [
            {"name": edge["node"]["name"], "url": edge["node"]["url"], "source": "producthunt"}
            for edge in post_edges
        ]
    except (httpx.HTTPError, KeyError, CompetitorSearchError) as exc:
        logger.error("competitor_search_failed", category=category, error=str(exc))
        return []
