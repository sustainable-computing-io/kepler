//go:build go1.17
// +build go1.17

package csvutil

import (
	"encoding/csv"
	"reflect"
	"testing"
)

// In Go1.17 csv.ParseError.Column became 1-indexed instead of 0-indexed.
// so we need this file for Go 1.17+.

var testUnmarshalInvalidFirstLineErr = &csv.ParseError{
	StartLine: 1,
	Line:      1,
	Column:    2,
	Err:       csv.ErrQuote,
}

var testUnmarshalInvalidSecondLineErr = &csv.ParseError{
	StartLine: 2,
	Line:      2,
	Column:    2,
	Err:       csv.ErrQuote,
}

var ptrUnexportedEmbeddedDecodeErr = errPtrUnexportedStruct(reflect.TypeOf(new(embedded)))

func TestUnmarshalGo117(t *testing.T) {
	t.Run("unmarshal type error message", func(t *testing.T) {
		expected := `csvutil: cannot unmarshal "field" into Go value of type int: field "X" line 3 column 3`
		err := Unmarshal([]byte("Y,X\n1,1\n2,field"), &[]A{})
		if err == nil {
			t.Fatal("want err not to be nil")
		}
		if err.Error() != expected {
			t.Errorf("want=%s; got %s", expected, err.Error())
		}
	})
}
