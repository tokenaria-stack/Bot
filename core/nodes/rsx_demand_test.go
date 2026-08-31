package nodes

import (
	"testing"
)

func TestRSXWorkClosure(t *testing.T) {
	if RSXWorkFromPlots([]string{"line_rsx"}) != NeedRSXCore {
		t.Fatal("line_rsx → Core only")
	}
	if RSXWorkFromPlots([]string{"line_rsx_signal"}) != NeedRSXCore {
		t.Fatal("signal → Core")
	}
	if RSXWorkFromPlots([]string{"woz_fast"}) != 0 {
		t.Fatal("woz plot is not RSX core")
	}
	if got := RSXWorkFromFactSources([]string{"rsx_tv_div"}); got != NeedRSXCore|NeedRSTV {
		t.Fatalf("tv div %#b", got)
	}
	if got := RSXWorkFromFactSources([]string{"rsx_tv_pivot"}); got != NeedRSXCore|NeedRSTV {
		t.Fatalf("tv pivot %#b", got)
	}
	if got := RSXWorkFromFactSources([]string{"rsx_fractal_div"}); got != NeedRSXCore|NeedRSFractal {
		t.Fatalf("fractal %#b", got)
	}
	if got := RSXWorkFromFactSources([]string{"rsx_zz_div"}); got != NeedRSXCore|NeedRSZZ {
		t.Fatalf("zz %#b", got)
	}
	if RSXWorkFromFactSources([]string{"nope"}) != 0 {
		t.Fatal("unknown source")
	}
}

func TestRSXWorkFromClientTriState(t *testing.T) {
	if RSXWorkFromClient(nil, nil) != RSXWorkAll {
		t.Fatal("omitted facts + empty plots → all (legacy)")
	}
	none := []string{}
	if RSXWorkFromClient([]string{"line_rsx"}, &none) != NeedRSXCore {
		t.Fatal("explicit [] facts → no families")
	}
	one := []string{"rsx_fractal_div"}
	if got := RSXWorkFromClient([]string{"woz_fast"}, &one); got != NeedRSXCore|NeedRSFractal {
		t.Fatalf("fractal-only %#b", got)
	}
}

func TestRSXWorkUnion(t *testing.T) {
	tv := []string{"rsx_tv_div"}
	fr := []string{"rsx_fractal_pivot"}
	got := RSXWorkFromClientSubscriptions([]RSXClientDemand{
		{Plots: []string{"line_rsx"}, Facts: &tv},
		{Plots: []string{"woz_slow"}, Facts: &fr},
	})
	want := NeedRSXCore | NeedRSTV | NeedRSFractal
	if got != want {
		t.Fatalf("union %#b want %#b", got, want)
	}
	if RSXWorkFromClientSubscriptions(nil) != 0 {
		t.Fatal("no clients")
	}
}
