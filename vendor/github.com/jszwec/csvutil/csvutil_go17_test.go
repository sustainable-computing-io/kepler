//go:build !go1.10
// +build !go1.10

package csvutil

import (
	"encoding/csv"
	"testing"
)

var testUnmarshalInvalidFirstLineErr = &csv.ParseError{
	Line:   1,
	Column: 1,
	Err:    csv.ErrQuote,
}

var testUnmarshalInvalidSecondLineErr = &csv.ParseError{
	Line:   2,
	Column: 1,
	Err:    csv.ErrQuote,
}

var ptrUnexportedEmbeddedDecodeErr error

func TestUnmarshalGo17(t *testing.T) {
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
