package market

import "testing"

func TestResearchTargetSpecC_DoesNotReplaceV1(t *testing.T) {
	v1, err := ResearchTargetSpec()
	if err != nil {
		t.Fatal(err)
	}
	c, err := ResearchTargetSpecC()
	if err != nil {
		t.Fatal(err)
	}
	id1, _ := v1.Identity()
	idc, _ := c.Identity()
	t.Log("target_v1", id1.Digest)
	t.Log("target_c", idc.Digest)
	if id1.Digest == idc.Digest {
		t.Fatal("C must be a new TargetDigest")
	}
	if v1.HorizonBars != 24 || v1.UpperATRMultiple != 1.5 || v1.LowerATRMultiple != 1 {
		t.Fatal("v1 target mutated")
	}
	if c.HorizonBars != 72 || c.UpperATRMultiple != 2 || c.LowerATRMultiple != 2 {
		t.Fatal("target C")
	}
}

func TestResearchFeatureSpec2_WidthAndV1Plan(t *testing.T) {
	s, err := ResearchFeatureSpec2()
	if err != nil {
		t.Fatal(err)
	}
	if s.Plan.VectorLen() != 64 || s.Q != 18 {
		t.Fatal(s.Plan.VectorLen(), s.Q)
	}
	tid, _ := s.Target.Identity()
	aid, _ := s.Analysis.Identity()
	fid, _ := s.Features.Identity()
	pid, _ := s.Plan.Identity()
	sid, _ := s.Identity()
	t.Log("target_c", tid.Digest)
	t.Log("analysis", aid.Digest)
	t.Log("features", fid.Digest)
	t.Log("plan", pid.Digest)
	t.Log("spec2", sid.Digest)
	v1, err := ResearchFeaturePlan("analysis:v2")
	if err != nil {
		t.Fatal(err)
	}
	if v1.VectorLen() != 4 {
		t.Fatal("v1 plan width")
	}
	if s.Analysis.Config.RSXSignal != 14 {
		t.Fatal("signal14")
	}
	v1a, err := AnalysisRecipeFromRSXSettings(ResearchRSXSettings(), true, false, analysisLogicV2)
	if err != nil {
		t.Fatal(err)
	}
	idOldA, _ := v1a.Identity()
	idNewA, _ := s.Analysis.Identity()
	if idOldA.Digest == idNewA.Digest {
		t.Fatal("signal 9 vs 14 must change AnalysisRecipe identity")
	}
}
