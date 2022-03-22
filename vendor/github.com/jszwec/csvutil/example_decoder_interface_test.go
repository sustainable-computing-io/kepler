package csvutil_test

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"log"

	"github.com/jszwec/csvutil"
)

// Value defines one record in the csv input. In this example it is important
// that Type field is defined before Value. Decoder reads headers and values
// in the same order as struct fields are defined.
type Value struct {
	Type  string      `csv:"type"`
	Value interface{} `csv:"value"`
}

func ExampleDecoder_interfaceValues() {
	// lets say our csv input defines variables with their types and values.
	data := []byte(`
type,value
string,string_value
int,10
`)

	dec, err := csvutil.NewDecoder(csv.NewReader(bytes.NewReader(data)))
	if err != nil {
		log.Fatal(err)
	}

	// we would like to read every variable and store their already parsed values
	// in the interface field. We can use Decoder.Map function to initialize
	// interface with proper values depending on the input.
	var value Value
	dec.Map = func(field, column string, v interface{}) string {
		if column == "type" {
			switch field {
			case "int": // csv input tells us that this variable contains an int.
				var n int
				value.Value = &n // lets initialize interface with an initialized int pointer.
			default:
				return field
			}
		}
		return field
	}

	for {
		value = Value{}
		if err := dec.Decode(&value); err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}

		if value.Type == "int" {
			// our variable type is int, Map func already initialized our interface
			// as int pointer, so we can safely cast it and use it.
			n, ok := value.Value.(*int)
			if !ok {
				log.Fatal("expected value to be *int")
			}
			fmt.Printf("value_type: %s; value: (%T) %d\n", value.Type, value.Value, *n)
		} else {
			fmt.Printf("value_type: %s; value: (%T) %v\n", value.Type, value.Value, value.Value)
		}
	}

	// Output:
	// value_type: string; value: (string) string_value
	// value_type: int; value: (*int) 10
}
