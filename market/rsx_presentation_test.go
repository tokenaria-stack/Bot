package market

import (
	"reflect"
	"testing"
)

func TestNoBackendRSXPresentationFields(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(HTFState{})
	if _, ok := typ.FieldByName("RSXColor"); ok {
		t.Fatalf("%s still carries RSXColor presentation state", typ.Name())
	}
}
