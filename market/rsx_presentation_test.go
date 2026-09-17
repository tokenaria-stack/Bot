package market

import (
	"reflect"
	"testing"
)

func TestNoBackendRSXPresentationFields(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(HTFState{})
	for _, name := range []string{"RSXColor", "RSXValue", "WozduhUp", "WozduhDown"} {
		if _, ok := typ.FieldByName(name); ok {
			t.Fatalf("%s still carries dead oscillator/presentation field %s", typ.Name(), name)
		}
	}
}
