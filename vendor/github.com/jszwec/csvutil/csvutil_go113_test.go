// +build go1.13

package csvutil

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestMarshalerError(t *testing.T) {
	testErr := errors.New("error")

	err := fmt.Errorf("some custom error: %w", &MarshalerError{
		Type:          reflect.TypeOf(1),
		MarshalerType: "csvutil.Marshaler",
		Err:           testErr,
	})

	if !errors.Is(err, testErr) {
		t.Errorf("expected errors.Is to return testErr")
	}
}
