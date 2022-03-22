package csvutil_test

import (
	"fmt"
	"log"

	"github.com/jszwec/csvutil"
)

func ExampleHeader() {
	type User struct {
		ID    int
		Name  string
		Age   int `csv:",omitempty"`
		State int `csv:"-"`
		City  string
		ZIP   string `csv:"zip_code"`
	}

	header, err := csvutil.Header(User{}, "csv")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(header)
	// Output:
	// [ID Name Age City zip_code]
}
