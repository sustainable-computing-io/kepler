package csvutil_test

import (
	"fmt"
	"log"
	"strings"

	"github.com/jszwec/csvutil"
)

type Strings []string

func (s Strings) MarshalCSV() ([]byte, error) {
	return []byte(strings.Join(s, ",")), nil // strings.Join takes []string but it will also accept Strings
}

type StringMap map[string]string

func (sm StringMap) MarshalCSV() ([]byte, error) {
	return []byte(fmt.Sprint(sm)), nil
}

func ExampleMarshal_sliceMap() {
	b, err := csvutil.Marshal([]struct {
		Strings Strings   `csv:"strings"`
		Map     StringMap `csv:"map"`
	}{
		{[]string{"a", "b"}, map[string]string{"a": "1"}}, // no type casting is required for slice and map aliases
		{Strings{"c", "d"}, StringMap{"b": "1"}},
	})

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s\n", b)

	// Output:
	// strings,map
	// "a,b",map[a:1]
	// "c,d",map[b:1]
}
