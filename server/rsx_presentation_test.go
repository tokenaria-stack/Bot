package server

import (
	"reflect"
	"testing"
)

func TestNoBackendRSXPresentationWireFields(t *testing.T) {
	t.Parallel()

	for _, typ := range []reflect.Type{
		reflect.TypeOf(tickPayload{}),
		reflect.TypeOf(ChartPoint{}),
		reflect.TypeOf(SimPoint{}),
		reflect.TypeOf(ChartOscillator{}),
	} {
		if _, ok := typ.FieldByName("RSXColor"); ok {
			t.Fatalf("%s still carries RSXColor", typ.Name())
		}
		if _, ok := typ.FieldByName("RSXMarker"); ok {
			t.Fatalf("%s still carries RSXMarker", typ.Name())
		}
		if _, ok := typ.FieldByName("Color"); ok {
			t.Fatalf("%s still carries Color presentation field", typ.Name())
		}
	}
}
