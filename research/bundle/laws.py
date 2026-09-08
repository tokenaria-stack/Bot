"""forecast-bundle-v1 composition laws (hashed; no BundleSpecDigest)."""

from __future__ import annotations

from typing import Any, Dict, List

from research.modelfit.spec import CLASS_ORDER

LOGIC_V1 = "forecast-bundle:portable-v1"
FORMAT = "forecast-bundle-v1"

RAW_MODEL_PROJECTION = "standardize_then_ordered_affine_logits"
COMPOSITION = "raw_logits_then_existing_recipe"
PUBLIC_OUTPUT = {
    "directional_rank": "signed_empirical_midrank",
    "probabilities": "calibrated_class_probabilities_length_3",
}


def bundle_payload(
    final_model_digest: str,
    recipe_digest: str,
    calibration_digest: str,
    rank_digest: str,
    feature_ids: List[str],
    feature_plan_digest: str,
    target_digest: str,
    label_logic_version: str,
) -> Dict[str, Any]:
    return {
        "bindings": {
            "calibration_content_digest": calibration_digest,
            "final_model_content_digest": final_model_digest,
            "rank_content_digest": rank_digest,
            "recipe_content_digest": recipe_digest,
        },
        "bundle_logic_version": LOGIC_V1,
        "composition": COMPOSITION,
        "format_version": FORMAT,
        "input_contract": {
            "feature_ids": list(feature_ids),
            "feature_plan_digest": feature_plan_digest,
        },
        "output_contract": {
            "class_order": list(CLASS_ORDER),
            "label_logic_version": label_logic_version,
            "target_digest": target_digest,
        },
        "public_output": dict(PUBLIC_OUTPUT),
        "raw_model_projection": RAW_MODEL_PROJECTION,
    }
