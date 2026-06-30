"""Unit tests for app.clustering.gap_score."""

from app.clustering.gap_score import (
    competitor_gap,
    compute_gap_score,
    frequency_score,
    trend_score,
)


def test_frequency_score_caps_at_ten():
    assert frequency_score(20) == 10.0
    assert frequency_score(100) == 10.0  # well above cap, still clamped


def test_frequency_score_scales_linearly_below_cap():
    assert frequency_score(10) == 5.0
    assert frequency_score(0) == 0.0


def test_trend_score_neutral_when_no_previous_period():
    assert trend_score(current_count=10, previous_count=None) == 5.0


def test_trend_score_neutral_when_previous_period_zero():
    assert trend_score(current_count=10, previous_count=0) == 5.0


def test_trend_score_increases_with_growth():
    score = trend_score(current_count=20, previous_count=10)  # +100%
    assert score > 5.0


def test_trend_score_decreases_with_decline():
    score = trend_score(current_count=5, previous_count=10)  # -50%
    assert score < 5.0


def test_trend_score_clamped_to_range():
    assert trend_score(current_count=1000, previous_count=1) <= 10.0
    assert trend_score(current_count=0, previous_count=1000) >= 0.0


def test_competitor_gap_max_when_no_competitors():
    assert competitor_gap(0) == 10.0


def test_competitor_gap_decreases_with_more_competitors():
    assert competitor_gap(1) == 8.0
    assert competitor_gap(5) == 0.0  # floored, not negative


def test_compute_gap_score_weights_components_correctly():
    # frequency_score(20)=10, intensity_avg=10, trend(None)=5, competitor_gap(0)=10
    # = 10*0.4 + 10*0.3 + 5*0.2 + 10*0.1 = 4 + 3 + 1 + 1 = 9.0
    score = compute_gap_score(
        post_count=20,
        intensity_avg=10,
        current_period_count=20,
        previous_period_count=None,
        competitor_count=0,
    )
    assert score == 9.0


def test_compute_gap_score_only_neutral_trend_contributes_when_everything_else_zero():
    # frequency_score(0)=0, intensity_avg=0, trend_score(0,0)=5 (neutral fallback),
    # competitor_gap(5)=0 (floored) -> 0*0.4 + 0*0.3 + 5*0.2 + 0*0.1 = 1.0
    score = compute_gap_score(
        post_count=0,
        intensity_avg=0,
        current_period_count=0,
        previous_period_count=0,
        competitor_count=5,
    )
    assert score == 1.0
