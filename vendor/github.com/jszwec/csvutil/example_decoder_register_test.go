package csvutil_test

import (
	"encoding/csv"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jszwec/csvutil"
)

type IntStruct struct {
	Value int
}

func (i *IntStruct) Scan(state fmt.ScanState, verb rune) error {
	switch verb {
	case 'd', 'v':
	default:
		return errors.New("unsupported verb")
	}

	t, err := state.Token(false, unicode.IsDigit)
	if err != nil {
		return err
	}

	n, err := strconv.Atoi(string(t))
	if err != nil {
		return err
	}
	*i = IntStruct{Value: n}
	return nil
}

func ExampleDecoder_Register() {
	type Foo struct {
		Time      time.Time `csv:"time"`
		Hex       int       `csv:"hex"`
		PtrHex    *int      `csv:"ptr_hex"`
		IntStruct IntStruct `csv:"int_struct"`
	}

	unmarshalInt := func(data []byte, n *int) error {
		v, err := strconv.ParseInt(string(data), 16, 64)
		if err != nil {
			return err
		}
		*n = int(v)
		return nil
	}

	unmarshalTime := func(data []byte, t *time.Time) error {
		tt, err := time.Parse(time.Kitchen, string(data))
		if err != nil {
			return err
		}
		*t = tt
		return nil
	}

	unmarshalScanner := func(data []byte, s fmt.Scanner) error {
		_, err := fmt.Sscan(string(data), s)
		return err
	}

	const data = `time,hex,ptr_hex,int_struct
12:00PM,f,a,34`

	r := csv.NewReader(strings.NewReader(data))
	dec, err := csvutil.NewDecoder(r)
	if err != nil {
		panic(err)
	}

	dec.Register(unmarshalInt)
	dec.Register(unmarshalTime)
	dec.Register(unmarshalScanner)

	var foos []Foo
	if err := dec.Decode(&foos); err != nil {
		fmt.Println("error:", err)
	}

	fmt.Printf("%s,%d,%d,%+v",
		foos[0].Time.Format(time.Kitchen),
		foos[0].Hex,
		*foos[0].PtrHex,
		foos[0].IntStruct,
	)

	// Output:
	// 12:00PM,15,10,{Value:34}
}
