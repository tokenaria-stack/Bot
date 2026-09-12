"""C-local RankSpec population pin. Math is RankSpec-v1; V1 pinned_rank_spec is not mutated."""

from __future__ import annotations

from research.rank.spec import RankSpec, pinned_rank_spec

RANK_POPULATION_C = "decision_fold_train_raw_logits"
PROJECTION_LOGIC = "forecast-recipe:composition-v1"


def decision_fold_rank_spec() -> RankSpec:
    base = pinned_rank_spec()
    return RankSpec(
        logic=base.logic,
        directional_evidence=base.directional_evidence,
        class_order=base.class_order,
        reference_population=RANK_POPULATION_C,
        reference_weighting=base.reference_weighting,
        empirical_law=base.empirical_law,
        tie_equality=base.tie_equality,
        tie_tolerance=base.tie_tolerance,
        tie_law=base.tie_law,
        cdf=base.cdf,
        query_left=base.query_left,
        query_right=base.query_right,
        signed_rank=base.signed_rank,
        dtype=base.dtype,
        finite=base.finite,
    )
