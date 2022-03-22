package csvutil_test

import (
	"fmt"

	"github.com/jszwec/csvutil"
)

type Status uint8

const (
	Unknown = iota
	Success
	Failure
)

func (s Status) MarshalCSV() ([]byte, error) {
	switch s {
	case Success:
		return []byte("success"), nil
	case Failure:
		return []byte("failure"), nil
	default:
		return []byte("unknown"), nil
	}
}

type Job struct {
	ID     int
	Status Status
}

func ExampleMarshal_customMarshalCSV() {
	jobs := []Job{
		{1, Success},
		{2, Failure},
	}

	b, err := csvutil.Marshal(jobs)
	if err != nil {
		fmt.Println("error:", err)
	}
	fmt.Println(string(b))

	// Output:
	// ID,Status
	// 1,success
	// 2,failure
}
