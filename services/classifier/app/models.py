"""Pydantic models for the LLM classification contract and persisted documents."""

from datetime import datetime
from enum import Enum
from typing import Optional

from pydantic import BaseModel, Field


class PainCategory(str, Enum):
    """Allowed categories for a classified pain signal, per the Technical Specification."""

    DEVTOOLS = "devtools"
    PRODUCTIVITY = "productivity"
    FINANCE = "finance"
    HEALTH = "health"
    EDUCATION = "education"
    MARKETING = "marketing"
    HR = "hr"
    LEGAL = "legal"
    OTHER = "other"


class ClassificationResult(BaseModel):
    """The exact JSON structure the LLM is instructed to return."""

    is_pain_signal: bool
    confidence: float = Field(ge=0.0, le=1.0)
    category: PainCategory
    pain_summary: str
    implied_solution: str
    intensity: int = Field(ge=1, le=10)
    target_user: str
    existing_solutions_mentioned: list[str] = Field(default_factory=list)
    reasoning: str


class ClassifiedPost(BaseModel):
    """Document shape written to MongoDB's classified_posts collection."""

    raw_post_id: str
    platform_id: str
    platform: str
    is_pain_signal: bool
    confidence: float
    category: PainCategory
    pain_summary: str
    implied_solution: str
    intensity: int
    target_user: str
    existing_solutions: list[str]
    embedding_id: Optional[str] = None
    cluster_id: Optional[str] = None
    classified_at: datetime
    llm_model: str
    token_cost_usd: float = 0.0
