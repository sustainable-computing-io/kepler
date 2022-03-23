package csvutil_test

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"log"

	"github.com/jszwec/csvutil"
)

type User struct {
	ID    int
	Name  string
	Age   int `csv:",omitempty"`
	State int `csv:"-"`
	City  string
	ZIP   string `csv:"zip_code"`
}

var userHeader []string

func init() {
	h, err := csvutil.Header(User{}, "csv")
	if err != nil {
		log.Fatal(err)
	}
	userHeader = h
}

func ExampleDecoder_decodingDataWithNoHeader() {
	data := []byte(`
1,John,27,la,90005
2,Bob,,ny,10005`)

	r := csv.NewReader(bytes.NewReader(data))

	dec, err := csvutil.NewDecoder(r, userHeader...)
	if err != nil {
		log.Fatal(err)
	}

	var users []User
	for {
		var u User

		if err := dec.Decode(&u); err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}
		users = append(users, u)
	}

	fmt.Printf("%+v", users)

	// Output:
	// [{ID:1 Name:John Age:27 State:0 City:la ZIP:90005} {ID:2 Name:Bob Age:0 State:0 City:ny ZIP:10005}]
}
