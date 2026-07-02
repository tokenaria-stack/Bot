package strategy

import (
	"testing"

	"trading_bot/exchange"
)

func TestDetectRedLineCrossGreenUp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		prevRed  float64
		prevGrn  float64
		curRed   float64
		curGrn   float64
		want     bool
	}{
		{
			name:    "cross up in zone",
			prevRed: 28, prevGrn: 30, curRed: 35, curGrn: 32,
			want: true,
		},
		{
			name:    "already above green",
			prevRed: 35, prevGrn: 30, curRed: 36, curGrn: 31,
			want: false,
		},
		{
			name:    "cross above zone ceiling",
			prevRed: 28, prevGrn: 30, curRed: 42, curGrn: 32,
			want: true,
		},
		{
			name:    "touch green no cross",
			prevRed: 29, prevGrn: 30, curRed: 30, curGrn: 30,
			want: false,
		},
		{
			name:    "cross from equal",
			prevRed: 30, prevGrn: 30, curRed: 31, curGrn: 30,
			want: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := detectRedLineCrossGreenUp(tc.prevRed, tc.prevGrn, tc.curRed, tc.curGrn)
			if got != tc.want {
				t.Fatalf("detectRedLineCrossGreenUp() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDetectRedLineCrossGreenDown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		prevRed  float64
		prevGrn  float64
		curRed   float64
		curGrn   float64
		want     bool
	}{
		{
			name:    "cross down in overbought zone",
			prevRed: 68, prevGrn: 65, curRed: 63, curGrn: 66,
			want: true,
		},
		{
			name:    "already below green",
			prevRed: 63, prevGrn: 66, curRed: 62, curGrn: 65,
			want: false,
		},
		{
			name:    "cross below zone floor",
			prevRed: 68, prevGrn: 65, curRed: 58, curGrn: 66,
			want: true,
		},
		{
			name:    "cross from equal at overbought",
			prevRed: 65, prevGrn: 65, curRed: 64, curGrn: 65,
			want: true,
		},
		{
			name:    "cross from above at floor edge",
			prevRed: 65, prevGrn: 62, curRed: 61, curGrn: 63,
			want: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := detectRedLineCrossGreenDown(tc.prevRed, tc.prevGrn, tc.curRed, tc.curGrn)
			if got != tc.want {
				t.Fatalf("detectRedLineCrossGreenDown() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDetectWozduxVolumeSpike(t *testing.T) {
	t.Parallel()

	if !detectWozduxVolumeSpikeUp(30, 50, 35) {
		t.Fatal("expected volume spike up")
	}
	if detectWozduxVolumeSpikeUp(30, 40, 35) {
		t.Fatal("delta below threshold should not register spike up")
	}
	if !detectWozduxVolumeSpikeDown(70, 50, 65) {
		t.Fatal("expected volume spike down")
	}
	if detectWozduxVolumeSpikeDown(70, 60, 55) {
		t.Fatal("delta below threshold should not register spike down")
	}
}

func TestDetectADFlow(t *testing.T) {
	t.Parallel()

	rising, falling := detectADFlow(105, []float64{100})
	if !rising || falling {
		t.Fatalf("rising=%v falling=%v, want true/false", rising, falling)
	}

	rising, falling = detectADFlow(95, []float64{100})
	if rising || !falling {
		t.Fatalf("rising=%v falling=%v, want false/true", rising, falling)
	}

	rising, falling = detectADFlow(110, []float64{100, 102, 104})
	if !rising || falling {
		t.Fatalf("rising=%v falling=%v, want true/false vs 3-bar avg", rising, falling)
	}
}

func TestMarker_RedLineCrossGreenUpInMarker(t *testing.T) {
	t.Parallel()

	klines := makeSyntheticKlines(60)
	analyst := NewMarker(klines, nil, "1m", "", ChaosConfig{AOFastPeriod: 5, AOSlowPeriod: 34})
	falcon := analyst.FalconSnapshot()
	if falcon.GreenLine <= 0 {
		t.Fatalf("expected dynamic GreenLine > 0, got %f", falcon.GreenLine)
	}
	if falcon.JurikRSX <= 0 {
		t.Fatalf("expected JurikRSX > 0, got %f", falcon.JurikRSX)
	}
}

func makeSyntheticKlines(n int) []exchange.Kline {
	klines := make([]exchange.Kline, n)
	price := 100.0
	for i := range klines {
		klines[i] = exchange.Kline{
			OpenTime: int64(i),
			Open:     price,
			High:     price + 1,
			Low:      price - 1,
			Close:    price,
			Volume:   1000,
		}
		price += 0.1
	}
	return klines
}
