package csvutil

import (
	"reflect"
	"testing"
)

func TestUnmarshal(t *testing.T) {
	fixture := []struct {
		desc string
		src  []byte
		in   interface{}
		out  interface{}
	}{
		{
			desc: "type with two records",
			src:  []byte("String,int\nstring1,1\nstring2,2"),
			in:   new([]TypeI),
			out: &[]TypeI{
				{"string1", 1},
				{"string2", 2},
			},
		},
		{
			desc: "pointer types with two records",
			src:  []byte("String,int\nstring1,1\nstring2,2"),
			in:   &[]*TypeI{},
			out: &[]*TypeI{
				{"string1", 1},
				{"string2", 2},
			},
		},
		{
			desc: "array - two records",
			src:  []byte("String,int\nstring1,1\nstring2,2"),
			in:   new([2]TypeI),
			out: &[2]TypeI{
				{"string1", 1},
				{"string2", 2},
			},
		},
		{
			desc: "array - pointer type with two records",
			src:  []byte("String,int\nstring1,1\nstring2,2"),
			in:   &[2]*TypeI{},
			out: &[2]*TypeI{
				{"string1", 1},
				{"string2", 2},
			},
		},
		{
			desc: "array - pointer type with two records size three",
			src:  []byte("String,int\nstring1,1\nstring2,2"),
			in:   &[3]*TypeI{},
			out: &[3]*TypeI{
				{"string1", 1},
				{"string2", 2},
				nil,
			},
		},
		{
			desc: "array - pointer type with two records size three - initialized",
			src:  []byte("String,int\nstring1,1\nstring2,2"),
			in:   &[3]*TypeI{{}, {}, {}},
			out: &[3]*TypeI{
				{"string1", 1},
				{"string2", 2},
				nil,
			},
		},
		{
			desc: "array - two records size three",
			src:  []byte("String,int\nstring1,1\nstring2,2"),
			in:   new([3]TypeI),
			out: &[3]TypeI{
				{"string1", 1},
				{"string2", 2},
				{},
			},
		},
		{
			desc: "array - two records size one",
			src:  []byte("String,int\nstring1,1\nstring2,2"),
			in:   new([1]TypeI),
			out: &[1]TypeI{
				{"string1", 1},
			},
		},
		{
			desc: "array - two records size zero",
			src:  []byte("String,int\nstring1,1\nstring2,2"),
			in:   new([0]TypeI),
			out:  &[0]TypeI{},
		},
		{
			desc: "quoted input",
			src:  []byte("\n\n\n\"String\",\"int\"\n\"string1,\n\",\"1\"\n\n\n\n\"string2\",\"2\""),
			in:   &[]TypeI{},
			out: &[]TypeI{
				{"string1,\n", 1},
				{"string2", 2},
			},
		},
		{
			desc: "quoted input - with endline",
			src:  []byte("\n\n\n\"String\",\"int\"\n\"string1,\n\",\"1\"\n\"string2\",\"2\"\n\n\n"),
			in:   &[]TypeI{},
			out: &[]TypeI{
				{"string1,\n", 1},
				{"string2", 2},
			},
		},
		{
			desc: "header only",
			src:  []byte("String,int\n"),
			in:   &[]TypeI{},
			out:  &[]TypeI{},
		},
		{
			desc: "no data",
			src:  []byte(""),
			in:   &[]TypeI{},
			out:  &[]TypeI{},
		},
	}

	for _, f := range fixture {
		t.Run(f.desc, func(t *testing.T) {
			if err := Unmarshal(f.src, f.in); err != nil {
				t.Fatalf("want err=nil; got %v", err)
			}

			if !reflect.DeepEqual(f.in, f.out) {
				t.Errorf("want out=%v; got %v", f.out, f.in)
			}

			out := reflect.ValueOf(f.out).Elem()
			in := reflect.ValueOf(f.in).Elem()
			if cout, cin := out.Cap(), in.Cap(); cout != cin {
				t.Errorf("want cap=%d; got %d", cout, cin)
			}
		})
	}

	t.Run("invalid data", func(t *testing.T) {
		type A struct{}

		fixtures := []struct {
			desc string
			data []byte
			err  error
		}{
			{
				desc: "invalid first line",
				data: []byte(`"`),
				err:  testUnmarshalInvalidFirstLineErr,
			},
			{
				desc: "invalid second line",
				data: []byte("line\n\""),
				err:  testUnmarshalInvalidSecondLineErr,
			},
		}

		for _, f := range fixtures {
			t.Run(f.desc, func(t *testing.T) {
				var a []A
				if err := Unmarshal(f.data, &a); !checkErr(f.err, err) {
					t.Errorf("want err=%v; got %v", f.err, err)
				}
			})
		}
	})

	t.Run("test invalid arguments", func(t *testing.T) {
		n := 1

		var fixtures = []struct {
			desc     string
			v        interface{}
			expected string
		}{
			{"nil interface", interface{}(nil), "csvutil: Unmarshal(nil)"},
			{"nil", nil, "csvutil: Unmarshal(nil)"},
			{"non pointer struct", struct{}{}, "csvutil: Unmarshal(non-pointer struct {})"},
			{"invalid type double pointer int", (**int)(nil), "csvutil: Unmarshal(invalid type **int)"},
			{"invalid type int", (*int)(nil), "csvutil: Unmarshal(invalid type *int)"},
			{"invalid initialized type int", &n, "csvutil: Unmarshal(invalid type *int)"},
			{"invalid type array of slice", (*[2][]TypeI)(nil), "csvutil: Unmarshal(invalid type *[2][]csvutil.TypeI)"},
			{"double array", &[2][1]TypeI{}, "csvutil: Unmarshal(invalid type *[2][1]csvutil.TypeI)"},
			{"double slice", &[][]TypeI{}, "csvutil: Unmarshal(invalid type *[][]csvutil.TypeI)"},
			{"triple slice", &[][][]TypeI{}, "csvutil: Unmarshal(invalid type *[][][]csvutil.TypeI)"},
			{"double ptr slice", &[]*[]TypeI{}, "csvutil: Unmarshal(invalid type *[]*[]csvutil.TypeI)"},
			{"int slice", &[]int{}, "csvutil: Unmarshal(invalid type *[]int)"},
		}

		for _, f := range fixtures {
			t.Run(f.desc, func(t *testing.T) {
				err := Unmarshal([]byte(""), f.v)
				if err == nil {
					t.Fatalf("want err != nil")
				}
				if got := err.Error(); got != f.expected {
					t.Errorf("want err=%s; got %s", f.expected, got)
				}
			})
		}
	})
}

func TestCountLines(t *testing.T) {
	fixtures := []struct {
		desc string
		data []byte
		out  int
	}{
		{
			desc: "three lines no endline",
			data: []byte(`line1,line1
line2,line2,
line3,line3`),
			out: 3,
		},
		{
			desc: "three lines",
			data: []byte(`line1,line1
line2,line2
line3,line3
`),
			out: 3,
		},
		{
			desc: "no data",
			data: []byte(``),
			out:  0,
		},
		{
			desc: "endline in a quoted string",
			data: []byte(`"line
""1""",line1
line2,"line   
  2"""
`),
			out: 2,
		},
		{
			desc: "empty lines",
			data: []byte("\n\nline1,line1\n\n\n\nline2,line2\n\n"),
			out:  2,
		},
		{
			desc: "1 line ending with quote",
			data: []byte(`"line1","line2"`),
			out:  1,
		},
		{
			desc: "1 line ending with quote - with endline",
			data: []byte(`"line1","line2"
`),
			out: 1,
		},
		{
			desc: "2 lines ending with quote",
			data: []byte(`"line1","line2"
line2,"line2"`),
			out: 2,
		},
	}

	for _, f := range fixtures {
		t.Run(f.desc, func(t *testing.T) {
			if out := countRecords(f.data); out != f.out {
				t.Errorf("want=%d; got %d", f.out, out)
			}
		})
	}
}

func TestMarshal(t *testing.T) {
	fixtures := []struct {
		desc string
		v    interface{}
		out  [][]string
		err  error
	}{
		{
			desc: "slice with basic type",
			v: []TypeI{
				{String: "string", Int: 10},
				{String: "", Int: 0},
			},
			out: [][]string{
				{"String", "int"},
				{"string", "10"},
				{"", ""},
			},
		},
		{
			desc: "array with basic type",
			v: [2]TypeI{
				{String: "string", Int: 10},
				{String: "", Int: 0},
			},
			out: [][]string{
				{"String", "int"},
				{"string", "10"},
				{"", ""},
			},
		},
		{
			desc: "slice with pointer type",
			v: []*TypeI{
				{String: "string", Int: 10},
				{String: "", Int: 0},
			},
			out: [][]string{
				{"String", "int"},
				{"string", "10"},
				{"", ""},
			},
		},
		{
			desc: "array with pointer type",
			v: [2]*TypeI{
				{String: "string", Int: 10},
				{String: "", Int: 0},
			},
			out: [][]string{
				{"String", "int"},
				{"string", "10"},
				{"", ""},
			},
		},
		{
			desc: "slice pointer",
			v: &[]*TypeI{
				{String: "string", Int: 10},
			},
			out: [][]string{
				{"String", "int"},
				{"string", "10"},
			},
		},
		{
			desc: "array pointer",
			v: &[1]*TypeI{
				{String: "string", Int: 10},
			},
			out: [][]string{
				{"String", "int"},
				{"string", "10"},
			},
		},
		{
			desc: "slice pointer wrapped in interface",
			v: func() (v interface{}) {
				v = &[]*TypeI{
					{String: "string", Int: 10},
				}
				return v
			}(),
			out: [][]string{
				{"String", "int"},
				{"string", "10"},
			},
		},
		{
			desc: "array pointer wrapped in interface",
			v: func() (v interface{}) {
				v = &[1]*TypeI{
					{String: "string", Int: 10},
				}
				return v
			}(),
			out: [][]string{
				{"String", "int"},
				{"string", "10"},
			},
		},
		{
			desc: "not a slice or array",
			v:    int64(1),
			err:  &InvalidMarshalError{Type: reflect.TypeOf(int64(1))},
		},
		{
			desc: "slice of non structs",
			v:    []int64{1},
			err:  &InvalidMarshalError{Type: reflect.TypeOf([]int64{})},
		},
		{
			desc: "array of non pointers",
			v:    [1]int64{1},
			err:  &InvalidMarshalError{Type: reflect.TypeOf([1]int64{})},
		},
		{
			desc: "nil value",
			v:    nilIface,
			err:  &InvalidMarshalError{Type: reflect.TypeOf(nilIface)},
		},
		{
			desc: "nil ptr value",
			v:    nilPtr,
			err:  &InvalidMarshalError{},
		},
		{
			desc: "nil interface ptr value",
			v:    nilIfacePtr,
			err:  &InvalidMarshalError{},
		},
		{
			desc: "marshal empty slice",
			v:    []TypeI{},
			out: [][]string{
				{"String", "int"},
			},
		},
		{
			desc: "marshal nil slice",
			v:    []TypeI(nil),
			out: [][]string{
				{"String", "int"},
			},
		},
		{
			desc: "marshal invalid struct type",
			v:    []InvalidType(nil),
			err:  &UnsupportedTypeError{Type: reflect.TypeOf(struct{}{})},
		},
	}

	for _, f := range fixtures {
		t.Run(f.desc, func(t *testing.T) {
			b, err := Marshal(f.v)
			if f.err != nil {
				if !checkErr(f.err, err) {
					t.Errorf("want err=%v; got %v", f.err, err)
				}
				return
			} else if err != nil {
				t.Errorf("want err=nil; got %v", err)
			}

			if expected := encodeCSV(t, f.out); string(b) != expected {
				t.Errorf("want %s; got %s", expected, string(b))
			}
		})
	}

	t.Run("invalid marshal error message", func(t *testing.T) {
		fixtures := []struct {
			desc     string
			expected string
			v        interface{}
		}{
			{
				desc:     "int64",
				expected: "csvutil: Marshal(invalid type int64)",
				v:        int64(1),
			},
			{
				desc:     "*int64",
				expected: "csvutil: Marshal(invalid type *int64)",
				v:        pint64(1),
			},
			{
				desc:     "[]int64",
				expected: "csvutil: Marshal(non struct slice []int64)",
				v:        []int64{},
			},
			{
				desc:     "[]int64",
				expected: "csvutil: Marshal(non struct slice *[]int64)",
				v:        &[]int64{},
			},
			{
				desc:     "[2]int64",
				expected: "csvutil: Marshal(non struct array *[2]int64)",
				v:        &[2]int64{},
			},
			{
				desc:     "[2]*int64",
				expected: "csvutil: Marshal(non struct array *[2]*int64)",
				v:        &[2]*int64{},
			},
			{
				desc:     "*[][]*TypeI",
				expected: "csvutil: Marshal(non struct slice *[][]*csvutil.TypeI)",
				v:        &[][]*TypeI{{}},
			},
			{
				desc:     "*[]*[]*TypeI",
				expected: "csvutil: Marshal(non struct slice *[]*[]*csvutil.TypeI)",
				v:        &[]*[]*TypeI{{}},
			},
			{
				desc:     "*[1][2]*TypeI",
				expected: "csvutil: Marshal(non struct array *[1][2]*csvutil.TypeI)",
				v:        &[1][2]*TypeI{{}},
			},
			{
				desc:     "*[1]*[2]*TypeI",
				expected: "csvutil: Marshal(non struct array *[1]*[2]*csvutil.TypeI)",
				v:        &[1]*[2]*TypeI{{}},
			},
			{
				desc:     "[1][2]TypeI",
				expected: "csvutil: Marshal(non struct array [1][2]csvutil.TypeI)",
				v:        [1][2]TypeI{{}},
			},
			{
				desc:     "nil interface",
				expected: "csvutil: Marshal(nil)",
				v:        nilIface,
			},
			{
				desc:     "nil ptr value",
				expected: "csvutil: Marshal(nil)",
				v:        nilPtr,
			},
			{
				desc:     "nil interface ptr value",
				expected: "csvutil: Marshal(nil)",
				v:        nilIfacePtr,
			},
		}

		for _, f := range fixtures {
			t.Run(f.desc, func(t *testing.T) {
				_, err := Marshal(f.v)
				if err == nil {
					t.Fatal("want err not to be nil")
				}
				if err.Error() != f.expected {
					t.Errorf("want=%s; got %s", f.expected, err.Error())
				}
			})
		}
	})
}

type TypeJ struct {
	String string `csv:"STR" json:"string"`
	Int    string `csv:"int" json:"-"`
	Embedded16
	Float string `csv:"float"`
}

type Embedded16 struct {
	Bool  bool  `json:"bool"`
	Uint  uint  `csv:"-"`
	Uint8 uint8 `json:"-"`
}

func TestHeader(t *testing.T) {
	fixture := []struct {
		desc   string
		v      interface{}
		tag    string
		header []string
		err    error
	}{
		{
			desc:   "simple type with default tag",
			v:      TypeG{},
			tag:    "",
			header: []string{"String", "Int"},
		},
		{
			desc:   "simple type",
			v:      TypeG{},
			tag:    "csv",
			header: []string{"String", "Int"},
		},
		{
			desc:   "simple type with ptr value",
			v:      &TypeG{},
			tag:    "csv",
			header: []string{"String", "Int"},
		},
		{
			desc:   "embedded types with conflict",
			v:      &TypeA{},
			tag:    "csv",
			header: []string{"string", "bool", "int"},
		},
		{
			desc:   "embedded type with tag",
			v:      &TypeB{},
			tag:    "csv",
			header: []string{"json", "string"},
		},
		{
			desc:   "embedded ptr type with tag",
			v:      &TypeD{},
			tag:    "csv",
			header: []string{"json", "string"},
		},
		{
			desc:   "embedded ptr type no tag",
			v:      &TypeC{},
			tag:    "csv",
			header: []string{"float", "string"},
		},
		{
			desc:   "type with omitempty tags",
			v:      TypeI{},
			tag:    "csv",
			header: []string{"String", "int"},
		},
		{
			desc:   "embedded with different json tag",
			v:      TypeJ{},
			tag:    "json",
			header: []string{"string", "bool", "Uint", "Float"},
		},
		{
			desc:   "embedded with default csv tag",
			v:      TypeJ{},
			tag:    "csv",
			header: []string{"STR", "int", "Bool", "Uint8", "float"},
		},
		{
			desc: "not a struct",
			v:    int(10),
			tag:  "csv",
			err:  &UnsupportedTypeError{Type: reflect.TypeOf(int(0))},
		},
		{
			desc: "slice",
			v:    []TypeJ{{}},
			tag:  "csv",
			err:  &UnsupportedTypeError{Type: reflect.TypeOf([]TypeJ{})},
		},
		{
			desc: "nil interface",
			v:    nilIface,
			tag:  "csv",
			err:  &UnsupportedTypeError{},
		},
		{
			desc:   "circular reference type",
			v:      &A{},
			tag:    "csv",
			header: []string{"Y", "X"},
		},
		{
			desc:   "conflicting fields",
			v:      &Embedded10{},
			tag:    "csv",
			header: []string{"Y"},
		},
		{
			desc: "inline - simple",
			v:    &Inline{},
			tag:  "csv",
			header: []string{
				"int",
				"Bool",
				"Uint8",
				"float",
				"prefix-STR",
				"prefix-int",
				"prefix-Bool",
				"prefix-Uint8",
				"prefix-float",
				"top-string",
				"STR",
			},
		},
		{
			desc:   "inline - chain",
			v:      &Inline5{},
			tag:    "csv",
			header: []string{"AS", "AAA", "S", "A"},
		},
		{
			desc:   "inline - top level",
			v:      &Inline8{},
			tag:    "csv",
			header: []string{"AA"},
		},
		{
			desc: "nil ptr of TypeF",
			v:    nilPtr,
			tag:  "csv",
			header: []string{
				"int",
				"pint",
				"int8",
				"pint8",
				"int16",
				"pint16",
				"int32",
				"pint32",
				"int64",
				"pint64",
				"uint",
				"puint",
				"uint8",
				"puint8",
				"uint16",
				"puint16",
				"uint32",
				"puint32",
				"uint64",
				"puint64",
				"float32",
				"pfloat32",
				"float64",
				"pfloat64",
				"string",
				"pstring",
				"bool",
				"pbool",
				"interface",
				"pinterface",
				"binary",
				"pbinary",
			},
		},
		{
			desc: "ptr to nil interface ptr of TypeF",
			v:    &nilIfacePtr,
			tag:  "csv",
			header: []string{
				"int",
				"pint",
				"int8",
				"pint8",
				"int16",
				"pint16",
				"int32",
				"pint32",
				"int64",
				"pint64",
				"uint",
				"puint",
				"uint8",
				"puint8",
				"uint16",
				"puint16",
				"uint32",
				"puint32",
				"uint64",
				"puint64",
				"float32",
				"pfloat32",
				"float64",
				"pfloat64",
				"string",
				"pstring",
				"bool",
				"pbool",
				"interface",
				"pinterface",
				"binary",
				"pbinary",
			},
		},
		{
			desc: "nil interface ptr of TypeF",
			v:    nilIfacePtr,
			tag:  "csv",
			header: []string{
				"int",
				"pint",
				"int8",
				"pint8",
				"int16",
				"pint16",
				"int32",
				"pint32",
				"int64",
				"pint64",
				"uint",
				"puint",
				"uint8",
				"puint8",
				"uint16",
				"puint16",
				"uint32",
				"puint32",
				"uint64",
				"puint64",
				"float32",
				"pfloat32",
				"float64",
				"pfloat64",
				"string",
				"pstring",
				"bool",
				"pbool",
				"interface",
				"pinterface",
				"binary",
				"pbinary",
			},
		},
		{
			desc: "ptr to nil interface",
			v:    &nilIface,
			err:  &UnsupportedTypeError{Type: reflect.ValueOf(&nilIface).Type().Elem()},
		},
	}

	for _, f := range fixture {
		t.Run(f.desc, func(t *testing.T) {
			h, err := Header(f.v, f.tag)

			if !checkErr(f.err, err) {
				t.Errorf("want err=%v; got %v", f.err, err)
			}

			if !reflect.DeepEqual(h, f.header) {
				t.Errorf("want header=%v; got %v", f.header, h)
			}
		})
	}

	t.Run("test nil value error message", func(t *testing.T) {
		const expected = "csvutil: unsupported type: nil"
		h, err := Header(nilIface, "")
		if h != nil {
			t.Errorf("want h=nil; got %v", h)
		}
		if err.Error() != expected {
			t.Errorf("want err=%s; got %s", expected, err.Error())
		}
	})
}

func TestParity(t *testing.T) {
	type A struct {
		Int      int
		Pint     *int
		OmitInt  int  `csv:",omitempty"`
		OmitPint *int `csv:",omitempty"`
	}

	in := []A{
		{
			Int:      0,
			Pint:     pint(0),
			OmitInt:  0,
			OmitPint: pint(0),
		},
		{
			Int:      1,
			Pint:     pint(1),
			OmitInt:  1,
			OmitPint: pint(1),
		},
		{
			Int:      0,
			Pint:     nil,
			OmitInt:  0,
			OmitPint: nil,
		},
	}

	b, err := Marshal(in)
	if err != nil {
		t.Fatalf("want err=nil; got %v", err)
	}

	var out []A
	if err := Unmarshal(b, &out); err != nil {
		t.Fatalf("want err=nil; got %v", err)
	}

	if !reflect.DeepEqual(in, out) {
		t.Errorf("want out=%v; got %v", in, out)
	}
}

func checkErr(expected, err error) bool {
	if expected == err {
		return true
	}

	eVal := reflect.New(reflect.TypeOf(expected))
	if !asError(err, eVal.Interface()) {
		return false
	}
	return reflect.DeepEqual(eVal.Elem().Interface(), expected)
}

// asError is a copy of errors.As to support older Go versions.
//
// This copy exists because we want to avoid dependencies like:
// "golang.org/x/xerrors"
func asError(err error, target interface{}) bool {
	if target == nil {
		panic("errors: target cannot be nil")
	}
	val := reflect.ValueOf(target)
	typ := val.Type()
	if typ.Kind() != reflect.Ptr || val.IsNil() {
		panic("errors: target must be a non-nil pointer")
	}
	targetType := typ.Elem()
	if targetType.Kind() != reflect.Interface && !targetType.Implements(errorType) {
		panic("errors: *target must be interface or implement error")
	}
	for err != nil {
		if reflect.TypeOf(err).AssignableTo(targetType) {
			val.Elem().Set(reflect.ValueOf(err))
			return true
		}
		if x, ok := err.(interface{ As(interface{}) bool }); ok && x.As(target) {
			return true
		}
		err = unwrap(err)
	}
	return false
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

// unwrap is a copy of errors.Unwrap for older Go versions and to avoid
// dependencies.
func unwrap(err error) error {
	u, ok := err.(interface {
		Unwrap() error
	})
	if !ok {
		return nil
	}
	return u.Unwrap()
}
