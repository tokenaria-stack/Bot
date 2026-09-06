// Package forecast defines FORECAST-SPEC-1: the contracts/identity/laws for
// the future evidence → probability engine.
//
// This is NOT a scoring engine. FeatureNodes (measurements) will feed a
// trained model that produces calibrated probabilities and an empirical
// RankPercentile — never arbitrary points.
//
// Permanent conceptual flow:
//
//	MARKET TRUTH
//	    ↓
//	ANALYTICAL TRUTH + FACTS   (AnalysisRuntime, owned by market/core/nodes)
//	    ↓
//	FeaturePlan                (this package: AnalysisRecipe + FeatureRecipe)
//	    ↓
//	feature vector
//	    ↓
//	EvidenceModel              (later: FORECAST-MODEL-1)
//	    ↓
//	calibrated probability + RankPercentile
//	    ↓
//	ForecastFrame
//	    ↓
//	Decision                   (later: DECISION-RESEARCH-1)
//
// # Position in the import DAG
//
// The existing Jeweler DAG is exchange → market → decision → execution.
// package forecast imports indicators for ATRSpec ownership (ATR-TRUTH-1)
// and data.NextBarOpen / CurrentBarOpen for primary-gap continuity (calendar-safe).
// It still imports NOTHING from exchange/market/decision/execution.
//
// # Governing law
//
// Maximum capability in identities and contracts. Minimum machinery until a
// real consumer exists. minimal implementation != limited architecture.
//
// # What this chapter defines
//
//   - MarketKey: venue + instrument + contract + timeframe (not just symbol+TF).
//   - LogicVersion / Digest / Identity: human key + full SHA-256 digest +
//     explicit semantic logic version. Friendly names are never identity.
//     Resolved (post-default) configuration is what gets digested.
//   - AnalysisRecipe: how analytical truth is computed. Owns periods/sources,
//     not paint, not weights, not TargetSpec.
//   - FeatureRecipe: which deterministic measurements are extracted. Reusable
//     across AnalysisRecipes (not permanently pinned to one).
//   - FeaturePlan: compiled bind of one AnalysisRecipe + one FeatureRecipe.
//     Bind fails (never zero-fills) if a requested feature's required
//     analytical capability is absent from the AnalysisRecipe.
//   - TargetSpec: first-passage UP_FIRST/DOWN_FIRST/TIMEOUT event, frozen
//     barriers, explicit dual-hit policy. AMBIGUOUS is a dataset status, not
//     a fourth model class.
//   - LabelSet: immutable JSONL outcomes for one FeatureTape + TargetSpec.
//     v1 is primary-TF first-passage. v2 may resolve primary DUAL_HIT with one
//     pinned same-family FinerTimeframe (LABEL-SET-1B).
//   - ForecastFrame + PublishForecastFrame: the fail-closed gate. No frame is
//     ever produced from a not-Ready fill or a nonfinite/out-of-range
//     probability set.
//
// # Ready vs absent (law #7)
//
// NotReady (insufficient warmup/history, incomplete ingress reconcile,
// invalid numerical state, bind failure) means: no forecast, no model
// evaluation, dst is undefined. It is a different concept from a feature
// being legitimately absent while the system IS Ready (e.g. no TV divergence
// exists right now): that is presence=0, age ignored — never a magic -1 and
// never confused with NotReady.
//
// # Nonfinite numeric law (law #9)
//
// Existing architecture may use NaN externally to mean "analytical output
// unavailable / sleeping" (e.g. sleeping RSX/Wozduh slots) — that remains
// valid and is NOT reopened here. What this chapter forbids is NaN/±Inf ever
// entering persistent analytical state, a FeaturePlan feature vector,
// scaler/model input, or a ForecastFrame's probability/rank fields. See
// ValidateFeatureVector and ValidateForecastFrame.
//
// # Fail closed (law #43)
//
// On any identity mismatch, bind failure, insufficient history, incomplete
// Frame reconcile, !Ready fill, or nonfinite/invalid probability: no
// ForecastFrame. Never zero-fill, never reuse the previous vector/frame,
// never fall back to a default recipe.
//
// # Deliberately deferred (not in this chapter)
//
// Python, training, model inference, multi-runtime
// map/registry/refcounting, config database, Save As UI, activation
// infrastructure (atomic.Pointer swap, EffectiveFrom seam ownership),
// Decision, backtest. See docs/ARCHITECTURE.md "Forecast Engine (FORECAST-
// SPEC-1)" for the full law list and docs/OPEN_DEBTS.md for the roadmap.
package forecast
