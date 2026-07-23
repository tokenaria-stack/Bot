package data

import "testing"

func TestIsFormingCloseTime(t *testing.T) {
	now := int64(1_000_000)
	cases := []struct {
		name      string
		closeTime int64
		want      bool
	}{
		{"closed_past", now - 1, false},
		{"forming_equal", now, true},
		{"forming_future", now + 40_000, true},
		{"unknown_zero", 0, false},
		{"negative", -1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsFormingCloseTime(tc.closeTime, now); got != tc.want {
				t.Fatalf("IsFormingCloseTime(%d,%d)=%v want %v", tc.closeTime, now, got, tc.want)
			}
		})
	}
}
