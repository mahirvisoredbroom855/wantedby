"""Typed environment configuration for the pain-classifier service."""

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """All configuration loaded from environment variables. No hardcoded defaults
    for secrets or connection strings — only safe, non-sensitive tuning knobs."""

    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

    mongo_uri: str
    redis_url: str
    qdrant_url: str
    qdrant_api_key: str = ""
    ollama_url: str
    database_url: str = ""

    anthropic_api_key: str = ""
    daily_token_budget_usd: float = 5.00

    classifier_workers: int = 3

    llm_model_dev: str = "llama3.2:3b"
    embedding_model: str = "nomic-embed-text"

    producthunt_api_token: str = ""


settings = Settings()
