package market

import (
	"reflect"
	"testing"
)

func TestNoBackendRSXPresentationFields(t *testing.T) {
	t.Parallel()

	for _, typ := range []reflect.Type{
		reflect.TypeOf(HTFState{}),
		reflect.TypeOf(BacktestChartPoint{}),
		reflect.TypeOf(BacktestSimPoint{}),
	} {
		if _, ok := typ.FieldByName("RSXColor"); ok {
			t.Fatalf("%s still carries RSXColor presentation state", typ.Name())
		}
		if typ.Name() == "HTFState" {
			continue
		}
		if _, ok := typ.FieldByName("Color"); ok {
			t.Fatalf("%s still carries Color presentation state", typ.Name())
		}
	}
}
