package csvutil_test

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"github.com/jszwec/csvutil"
)

func ExampleEncoder_Encode_streaming() {
	type Address struct {
		City    string
		Country string
	}

	type User struct {
		Name string
		Address
		Age int `csv:"age,omitempty"`
	}

	users := []User{
		{Name: "John", Address: Address{"Boston", "USA"}, Age: 26},
		{Name: "Bob", Address: Address{"LA", "USA"}, Age: 27},
		{Name: "Alice", Address: Address{"SF", "USA"}},
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	enc := csvutil.NewEncoder(w)

	for _, u := range users {
		if err := enc.Encode(u); err != nil {
			fmt.Println("error:", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		fmt.Println("error:", err)
	}

	fmt.Println(buf.String())

	// Output:
	// Name,City,Country,age
	// John,Boston,USA,26
	// Bob,LA,USA,27
	// Alice,SF,USA,
}

func ExampleEncoder_Encode_all() {
	type Address struct {
		City    string
		Country string
	}

	type User struct {
		Name string
		Address
		Age int `csv:"age,omitempty"`
	}

	users := []User{
		{Name: "John", Address: Address{"Boston", "USA"}, Age: 26},
		{Name: "Bob", Address: Address{"LA", "USA"}, Age: 27},
		{Name: "Alice", Address: Address{"SF", "USA"}},
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := csvutil.NewEncoder(w).Encode(users); err != nil {
		fmt.Println("error:", err)
	}

	w.Flush()
	if err := w.Error(); err != nil {
		fmt.Println("error:", err)
	}

	fmt.Println(buf.String())

	// Output:
	// Name,City,Country,age
	// John,Boston,USA,26
	// Bob,LA,USA,27
	// Alice,SF,USA,
}

func ExampleEncoder_EncodeHeader() {
	type User struct {
		Name string
		Age  int `csv:"age,omitempty"`
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	enc := csvutil.NewEncoder(w)

	if err := enc.EncodeHeader(User{}); err != nil {
		fmt.Println("error:", err)
	}

	w.Flush()
	if err := w.Error(); err != nil {
		fmt.Println("error:", err)
	}

	fmt.Println(buf.String())

	// Output:
	// Name,age
}

func ExampleEncoder_Encode_inline() {
	type Address struct {
		Street string `csv:"street"`
		City   string `csv:"city"`
	}

	type User struct {
		Name        string  `csv:"name"`
		Address     Address `csv:",inline"`
		HomeAddress Address `csv:"home_address_,inline"`
		WorkAddress Address `csv:"work_address_,inline"`
		Age         int     `csv:"age,omitempty"`
	}

	users := []User{
		{
			Name:        "John",
			Address:     Address{"Washington", "Boston"},
			HomeAddress: Address{"Boylston", "Boston"},
			WorkAddress: Address{"River St", "Cambridge"},
			Age:         26,
		},
	}

	b, err := csvutil.Marshal(users)
	if err != nil {
		fmt.Println("error:", err)
	}

	fmt.Printf("%s\n", b)

	// Output:
	// name,street,city,home_address_street,home_address_city,work_address_street,work_address_city,age
	// John,Washington,Boston,Boylston,Boston,River St,Cambridge,26
}

func ExampleEncoder_Register() {
	type Foo struct {
		Time   time.Time     `csv:"time"`
		Hex    int           `csv:"hex"`
		PtrHex *int          `csv:"ptr_hex"`
		Buffer *bytes.Buffer `csv:"buffer"`
	}

	foos := []Foo{
		{
			Time:   time.Date(2020, 6, 20, 12, 0, 0, 0, time.UTC),
			Hex:    15,
			Buffer: bytes.NewBufferString("hello"),
		},
	}

	marshalInt := func(n *int) ([]byte, error) {
		if n == nil {
			return []byte("NULL"), nil
		}
		return strconv.AppendInt(nil, int64(*n), 16), nil
	}

	marshalTime := func(t time.Time) ([]byte, error) {
		return t.AppendFormat(nil, time.Kitchen), nil
	}

	// all fields which implement String method will use this, unless their
	// concrete type was already overriden.
	marshalStringer := func(s fmt.Stringer) ([]byte, error) {
		return []byte(s.String()), nil
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	enc := csvutil.NewEncoder(w)

	enc.Register(marshalInt)
	enc.Register(marshalTime)
	enc.Register(marshalStringer)

	if err := enc.Encode(foos); err != nil {
		fmt.Println("error:", err)
	}

	w.Flush()
	if err := w.Error(); err != nil {
		fmt.Println("error:", err)
	}

	fmt.Println(buf.String())

	// Output:
	// time,hex,ptr_hex,buffer
	// 12:00PM,f,NULL,hello
}
