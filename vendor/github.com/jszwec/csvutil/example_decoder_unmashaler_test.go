package csvutil_test

import (
	"fmt"
	"strconv"

	"github.com/jszwec/csvutil"
)

type Bar int

func (b *Bar) UnmarshalCSV(data []byte) error {
	n, err := strconv.Atoi(string(data))
	*b = Bar(n)
	return err
}

type Foo struct {
	Int int `csv:"int"`
	Bar Bar `csv:"bar"`
}

func ExampleDecoder_customUnmarshalCSV() {
	var csvInput = []byte(`
int,bar
5,10
6,11`)

	var foos []Foo
	if err := csvutil.Unmarshal(csvInput, &foos); err != nil {
		fmt.Println("error:", err)
	}

	fmt.Printf("%+v", foos)

	// Output:
	// [{Int:5 Bar:10} {Int:6 Bar:11}]
}
