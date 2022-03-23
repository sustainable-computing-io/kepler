package csvutil_test

import (
	"fmt"
	"time"

	"github.com/jszwec/csvutil"
)

func ExampleMarshal() {
	type Address struct {
		City    string
		Country string
	}

	type User struct {
		Name string
		Address
		Age       int `csv:"age,omitempty"`
		CreatedAt time.Time
	}

	users := []User{
		{
			Name:      "John",
			Address:   Address{"Boston", "USA"},
			Age:       26,
			CreatedAt: time.Date(2010, 6, 2, 12, 0, 0, 0, time.UTC),
		},
		{
			Name:    "Alice",
			Address: Address{"SF", "USA"},
		},
	}

	b, err := csvutil.Marshal(users)
	if err != nil {
		fmt.Println("error:", err)
	}
	fmt.Println(string(b))

	// Output:
	// Name,City,Country,age,CreatedAt
	// John,Boston,USA,26,2010-06-02T12:00:00Z
	// Alice,SF,USA,,0001-01-01T00:00:00Z
}
