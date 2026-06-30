"""Gap score computation, per the spec's formula with documented normalization choices.

gap_score = (frequency_score * 0.4) + (intensity_avg * 0.3)
          + (trend_score * 0.2) + (competitor_gap * 0.1)

The spec defines the weights but leaves each component's 0-10 normalization to the
implementation. The choices below are reasonable defaults, not spec-mandated --
flagged explicitly since a different normalization is a legitimate alternative.
"""

FREQUENCY_NORMALIZATION_CAP = 20  # post_count >= this maxes frequency_score at 10
NEUTRAL_TREND_SCORE = 5.0  # used when there's no prior period to compare against
COMPETITOR_PENALTY_PER_MATCH = 2.0  # each known competitor reduces gap by this much


def frequency_score(post_count: int) -> float:
    """Normalizes post count to 0-10. Capped at FREQUENCY_NORMALIZATION_CAP posts."""
    return min(post_count / FREQUENCY_NORMALIZATION_CAP, 1.0) * 10


def trend_score(current_count: int, previous_count: int | None) -> float:
    """Normalizes period-over-period growth to 0-10, centered at 5 (no change).
    Returns the neutral score if there's no previous period to compare against
    (e.g. a cluster's first-ever run).
    """
    if previous_count is None or previous_count == 0:
        return NEUTRAL_TREND_SCORE

    pct_change = (current_count - previous_count) / previous_count
    # Map [-100%, +100%+] onto [0, 10], clamped, centered at 5 for 0% change.
    score = 5.0 + (pct_change * 5.0)
    return max(0.0, min(10.0, score))


def competitor_gap(competitor_count: int) -> float:
    """Fewer known competitors implies a bigger market gap. 0 competitors -> 10
    (max gap). Each additional competitor reduces the score; floors at 0.
    """
    return max(0.0, 10.0 - (competitor_count * COMPETITOR_PENALTY_PER_MATCH))


def compute_gap_score(
    post_count: int,
    intensity_avg: float,
    current_period_count: int,
    previous_period_count: int | None,
    competitor_count: int,
) -> float:
    """Computes the final weighted gap score per the spec's formula."""
    freq = frequency_score(post_count)
    trend = trend_score(current_period_count, previous_period_count)
    comp_gap = competitor_gap(competitor_count)

    return (freq * 0.4) + (intensity_avg * 0.3) + (trend * 0.2) + (comp_gap * 0.1)
