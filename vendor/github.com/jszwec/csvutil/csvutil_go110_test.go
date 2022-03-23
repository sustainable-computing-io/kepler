//go:build !go1.17 && go1.10
// +build !go1.17,go1.10

package csvutil

import (
	"encoding/csv"
	"reflect"
	"testing"
)

var testUnmarshalInvalidFirstLineErr = &csv.ParseError{
	StartLine: 1,
	Line:      1,
	Column:    1,
	Err:       csv.ErrQuote,
}

var testUnmarshalInvalidSecondLineErr = &csv.ParseError{
	StartLine: 2,
	Line:      2,
	Column:    1,
	Err:       csv.ErrQuote,
}

var ptrUnexportedEmbeddedDecodeErr = errPtrUnexportedStruct(reflect.TypeOf(new(embedded)))

func TestUnmarshalGo110(t *testing.T) {
	t.Run("unmarshal type error message", func(t *testing.T) {
		expected := `csvutil: cannot unmarshal "field" into Go value of type int: field "X"`
		err := Unmarshal([]byte("Y,X\n1,1\n2,field"), &[]A{})
		if err == nil {
			t.Fatal("want err not to be nil")
		}
		if err.Error() != expected {
			t.Errorf("want=%s; got %s", expected, err.Error())
		}
	})
}
