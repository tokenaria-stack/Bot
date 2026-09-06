package forecast

import (
	"testing"

	"trading_bot/data"
)

func TestHorizonEnd_StrictHops(t *testing.T) {
	t.Parallel()
	at := int64(1_699_999_200_000)
	one, err := data.NextBarOpen(at, "15m")
	if err != nil {
		t.Fatal(err)
	}
	got, err := HorizonEnd(at, "15m", 1)
	if err != nil {
		t.Fatal(err)
	}
	if got != one {
		t.Fatalf("H=1 HorizonEnd=%d want NextBarOpen=%d", got, one)
	}
	two, err := data.NextBarOpen(one, "15m")
	if err != nil {
		t.Fatal(err)
	}
	got, err = HorizonEnd(at, "15m", 2)
	if err != nil {
		t.Fatal(err)
	}
	if got != two {
		t.Fatalf("H=2 HorizonEnd=%d want %d", got, two)
	}
	same, err := HorizonEnd(at, "15m", 0)
	if err != nil || same != at {
		t.Fatalf("H=0 got %d %v", same, err)
	}
}
