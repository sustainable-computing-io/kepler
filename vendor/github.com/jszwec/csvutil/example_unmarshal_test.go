package csvutil_test

import (
	"fmt"
	"time"

	"github.com/jszwec/csvutil"
)

func ExampleUnmarshal() {
	var csvInput = []byte(`
name,age,CreatedAt
jacek,26,2012-04-01T15:00:00Z
john,,0001-01-01T00:00:00Z`,
	)

	type User struct {
		Name      string `csv:"name"`
		Age       int    `csv:"age,omitempty"`
		CreatedAt time.Time
	}

	var users []User
	if err := csvutil.Unmarshal(csvInput, &users); err != nil {
		fmt.Println("error:", err)
	}

	for _, u := range users {
		fmt.Printf("%+v\n", u)
	}

	// Output:
	// {Name:jacek Age:26 CreatedAt:2012-04-01 15:00:00 +0000 UTC}
	// {Name:john Age:0 CreatedAt:0001-01-01 00:00:00 +0000 UTC}
}
