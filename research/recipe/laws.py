"""forecast-recipe-v1 composition laws (hashed; no RecipeSpecDigest family)."""

from __future__ import annotations

from typing import Any, Dict, Tuple

from research.modelfit.spec import CLASS_ORDER, LOGIC_V1 as MODEL_LOGIC_V1

LOGIC_V1 = "forecast-recipe:composition-v1"
FORMAT = "forecast-recipe-v1"

PROJECTION: Dict[str, str] = {
    "beta_zero_law": "uniform_one_third",
    "branch_independence": "independent",
    "probability_projection": "calibrated_temperature_softmax",
    "rank_projection": "raw_up_minus_down_then_rank1_query",
    "stable_softmax_law": "max_shift_then_scale_then_exp_then_normalize",
}

OUTPUTS: Dict[str, str] = {
    "directional_rank": "signed_empirical_midrank",
    "probabilities": "calibrated_class_probabilities_length_3",
}


def composition_payload(
    model_spec_digest: str,
    oof_logits_digest: str,
    calibration_digest: str,
    rank_digest: str,
    class_order: Tuple[str, ...],
) -> Dict[str, Any]:
    return {
        "base_model": {
            "model_logic_version": MODEL_LOGIC_V1,
            "model_spec_digest": model_spec_digest,
        },
        "calibration": {"content_digest": calibration_digest},
        "class_order": list(class_order),
        "format_version": FORMAT,
        "outputs": dict(OUTPUTS),
        "projection": dict(PROJECTION),
        "rank": {"content_digest": rank_digest},
        "recipe_logic_version": LOGIC_V1,
        "research_source": {"oof_logits_content_digest": oof_logits_digest},
    }


def require_class_order(order) -> None:
    if list(order) != list(CLASS_ORDER):
        raise RuntimeError("recipe: class_order mismatch")
