"""RankSpec-v1 and RankSpecDigest (rules only)."""

from __future__ import annotations

import hashlib
import json
from dataclasses import dataclass
from typing import Any, Dict, Tuple

from research.modelfit.spec import CLASS_ORDER

LOGIC_V1 = "rank:empirical-directional-v1"


@dataclass(frozen=True)
class RankSpec:
    logic: str
    directional_evidence: str
    class_order: Tuple[str, ...]
    reference_population: str
    reference_weighting: str
    empirical_law: str
    tie_equality: str
    tie_tolerance: str
    tie_law: str
    cdf: str
    query_left: str
    query_right: str
    signed_rank: str
    dtype: str
    finite: str

    def payload(self) -> Dict[str, Any]:
        return {
            "cdf": self.cdf,
            "class_order": list(self.class_order),
            "directional_evidence": self.directional_evidence,
            "dtype": self.dtype,
            "empirical_law": self.empirical_law,
            "finite": self.finite,
            "logic": self.logic,
            "query_left": self.query_left,
            "query_right": self.query_right,
            "reference_population": self.reference_population,
            "reference_weighting": self.reference_weighting,
            "signed_rank": self.signed_rank,
            "tie_equality": self.tie_equality,
            "tie_law": self.tie_law,
            "tie_tolerance": self.tie_tolerance,
        }

    def digest(self) -> bytes:
        blob = json.dumps(self.payload(), sort_keys=True, separators=(",", ":"), ensure_ascii=True)
        return hashlib.sha256(blob.encode("utf-8")).digest()

    def digest_hex(self) -> str:
        return self.digest().hex()

    def as_json(self) -> Dict[str, Any]:
        return self.payload()


def pinned_rank_spec() -> RankSpec:
    return RankSpec(
        logic=LOGIC_V1,
        directional_evidence="logit[UP_FIRST] - logit[DOWN_FIRST]",
        class_order=CLASS_ORDER,
        reference_population="all_rows_of_source_oof_logits_v1",
        reference_weighting="uniform_rows",
        empirical_law="exact_sorted_float64_sample",
        tie_equality="exact_float64_numeric_equality",
        tie_tolerance="none",
        tie_law="midrank",
        cdf="(count_less + 0.5 * count_equal) / N",
        query_left="first_index_i_such_that_sorted_d_i_ge_d_else_N",
        query_right="first_index_i_such_that_sorted_d_i_gt_d_else_N",
        signed_rank="2 * CDF - 1",
        dtype="float64",
        finite="required",
    )


def spec_from_json(d: Dict[str, Any]) -> RankSpec:
    return RankSpec(
        logic=d["logic"],
        directional_evidence=d["directional_evidence"],
        class_order=tuple(d["class_order"]),
        reference_population=d["reference_population"],
        reference_weighting=d["reference_weighting"],
        empirical_law=d["empirical_law"],
        tie_equality=d["tie_equality"],
        tie_tolerance=d["tie_tolerance"],
        tie_law=d["tie_law"],
        cdf=d["cdf"],
        query_left=d["query_left"],
        query_right=d["query_right"],
        signed_rank=d["signed_rank"],
        dtype=d["dtype"],
        finite=d["finite"],
    )
