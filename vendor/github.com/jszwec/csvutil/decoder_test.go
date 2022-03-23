package csvutil

import (
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

var Binary = []byte("binary-data")

var EncodedBinary = base64.StdEncoding.EncodeToString(Binary)

var BinaryLarge = bytes.Repeat([]byte("1"), 128*1024)

var EncodedBinaryLarge = base64.StdEncoding.EncodeToString(BinaryLarge)

type Float float64

type Enum uint8

const (
	EnumDefault = iota
	EnumFirst
	EnumSecond
)

func (e Enum) MarshalCSV() ([]byte, error) {
	switch e {
	case EnumFirst:
		return []byte("first"), nil
	case EnumSecond:
		return []byte("second"), nil
	default:
		return []byte("default"), nil
	}
}

func (e *Enum) UnmarshalCSV(data []byte) error {
	s := string(data)
	switch s {
	case "first":
		*e = EnumFirst
	case "second":
		*e = EnumSecond
	default:
		*e = EnumDefault
	}
	return nil
}

type ValueRecUnmarshaler struct {
	S *string
}

func (u ValueRecUnmarshaler) UnmarshalCSV(data []byte) error {
	*u.S = string(data)
	return nil
}

func (u ValueRecUnmarshaler) Scan(data []byte) error {
	*u.S = "scan: "
	*u.S += string(data)
	return nil
}

type ValueRecTextUnmarshaler struct {
	S *string
}

func (u ValueRecTextUnmarshaler) UnmarshalText(text []byte) error {
	*u.S = string(text)
	return nil
}

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

type EnumType struct {
	Enum Enum `csv:"enum"`
}

type Embedded1 struct {
	String string  `csv:"string"`
	Float  float64 `csv:"float"`
}

type Embedded2 struct {
	Float float64 `csv:"float"`
	Bool  bool    `csv:"bool"`
}

type Embedded3 map[string]string

func (e *Embedded3) UnmarshalCSV(s []byte) error {
	return json.Unmarshal(s, e)
}

func (e Embedded3) MarshalCSV() ([]byte, error) {
	return json.Marshal(e)
}

type Embedded4 interface{}

type Embedded5 struct {
	Embedded6
	Embedded7
	Embedded8
}

type Embedded6 struct {
	X int
}

type Embedded7 Embedded6

type Embedded8 struct {
	Embedded9
}

type Embedded9 struct {
	X int
	Y int
}

type Embedded10 struct {
	Embedded11
	Embedded12
	Embedded13
}

type Embedded11 struct {
	Embedded6
}

type Embedded12 struct {
	Embedded6
}

type Embedded13 struct {
	Embedded8
}

type Embedded17 struct {
	*Embedded18
}

type Embedded18 struct {
	X *float64
	Y *float64
}

type TypeA struct {
	Embedded1
	String string `csv:"string"`
	Embedded2
	Int int `csv:"int"`
}

type TypeB struct {
	Embedded3 `csv:"json"`
	String    string `csv:"string"`
}

type TypeC struct {
	*Embedded1
	String string `csv:"string"`
}

type TypeD struct {
	*Embedded3 `csv:"json"`
	String     string `csv:"string"`
}

type TypeE struct {
	String **string `csv:"string"`
	Int    *int     `csv:"int"`
}

type TypeF struct {
	Int      int          `csv:"int" custom:"int"`
	Pint     *int         `csv:"pint" custom:"pint"`
	Int8     int8         `csv:"int8" custom:"int8"`
	Pint8    *int8        `csv:"pint8" custom:"pint8"`
	Int16    int16        `csv:"int16" custom:"int16"`
	Pint16   *int16       `csv:"pint16" custom:"pint16"`
	Int32    int32        `csv:"int32" custom:"int32"`
	Pint32   *int32       `csv:"pint32" custom:"pint32"`
	Int64    int64        `csv:"int64" custom:"int64"`
	Pint64   *int64       `csv:"pint64" custom:"pint64"`
	UInt     uint         `csv:"uint" custom:"uint"`
	Puint    *uint        `csv:"puint" custom:"puint"`
	Uint8    uint8        `csv:"uint8" custom:"uint8"`
	Puint8   *uint8       `csv:"puint8" custom:"puint8"`
	Uint16   uint16       `csv:"uint16" custom:"uint16"`
	Puint16  *uint16      `csv:"puint16" custom:"puint16"`
	Uint32   uint32       `csv:"uint32" custom:"uint32"`
	Puint32  *uint32      `csv:"puint32" custom:"puint32"`
	Uint64   uint64       `csv:"uint64" custom:"uint64"`
	Puint64  *uint64      `csv:"puint64" custom:"puint64"`
	Float32  float32      `csv:"float32" custom:"float32"`
	Pfloat32 *float32     `csv:"pfloat32" custom:"pfloat32"`
	Float64  float64      `csv:"float64" custom:"float64"`
	Pfloat64 *float64     `csv:"pfloat64" custom:"pfloat64"`
	String   string       `csv:"string" custom:"string"`
	PString  *string      `csv:"pstring" custom:"pstring"`
	Bool     bool         `csv:"bool" custom:"bool"`
	Pbool    *bool        `csv:"pbool" custom:"pbool"`
	V        interface{}  `csv:"interface" custom:"interface"`
	Pv       *interface{} `csv:"pinterface" custom:"pinterface"`
	Binary   []byte       `csv:"binary" custom:"binary"`
	PBinary  *[]byte      `csv:"pbinary" custom:"pbinary"`
}

type TypeG struct {
	String      string
	Int         int
	Float       float64 `csv:"-"`
	unexported1 int
	unexported2 int `csv:"unexported2"`
}

type TypeI struct {
	String string `csv:",omitempty"`
	Int    int    `csv:"int,omitempty"`
}

type TypeK struct {
	*TypeL
}

type TypeL struct {
	String string
	Int    int `csv:",omitempty"`
}

type Unmarshalers struct {
	CSVUnmarshaler      CSVUnmarshaler      `csv:"csv"`
	PCSVUnmarshaler     *CSVUnmarshaler     `csv:"pcsv"`
	TextUnmarshaler     TextUnmarshaler     `csv:"text"`
	PTextUnmarshaler    *TextUnmarshaler    `csv:"ptext"`
	CSVTextUnmarshaler  CSVTextUnmarshaler  `csv:"csv-text"`
	PCSVTextUnmarshaler *CSVTextUnmarshaler `csv:"pcsv-text"`
}

type EmbeddedUnmarshalers struct {
	CSVUnmarshaler     `csv:"csv"`
	TextUnmarshaler    `csv:"text"`
	CSVTextUnmarshaler `csv:"csv-text"`
}

type EmbeddedPtrUnmarshalers struct {
	*CSVUnmarshaler     `csv:"csv"`
	*TextUnmarshaler    `csv:"text"`
	*CSVTextUnmarshaler `csv:"csv-text"`
}

type CSVUnmarshaler struct {
	String string `csv:"string"`
}

func (t *CSVUnmarshaler) UnmarshalCSV(s []byte) error {
	t.String = "unmarshalCSV:" + string(s)
	return nil
}

type TextUnmarshaler struct {
	String string `csv:"string"`
}

func (t *TextUnmarshaler) UnmarshalText(text []byte) error {
	t.String = "unmarshalText:" + string(text)
	return nil
}

type CSVTextUnmarshaler struct {
	String string `csv:"string"`
}

func (t *CSVTextUnmarshaler) UnmarshalCSV(s []byte) error {
	t.String = "unmarshalCSV:" + string(s)
	return nil
}

func (t *CSVTextUnmarshaler) UnmarshalText(text []byte) error {
	t.String = "unmarshalText:" + string(text)
	return nil
}

type TypeWithInvalidField struct {
	String TypeI `csv:"string"`
}

type InvalidType struct {
	String struct{}
}

type TagPriority struct {
	Foo int
	Bar int `csv:"Foo"`
}

type embedded struct {
	Foo int `csv:"foo"`
	bar int `csv:"bar"`
}

type UnexportedEmbedded struct {
	embedded
}

type UnexportedEmbeddedPtr struct {
	*embedded
}

type A struct {
	B
	X int
}

type B struct {
	*A
	Y int
}

var Int = 10
var String = "string"
var PString = &String
var TypeISlice []TypeI

func pint(n int) *int                       { return &n }
func pint8(n int8) *int8                    { return &n }
func pint16(n int16) *int16                 { return &n }
func pint32(n int32) *int32                 { return &n }
func pint64(n int64) *int64                 { return &n }
func puint(n uint) *uint                    { return &n }
func puint8(n uint8) *uint8                 { return &n }
func puint16(n uint16) *uint16              { return &n }
func puint32(n uint32) *uint32              { return &n }
func puint64(n uint64) *uint64              { return &n }
func pfloat32(f float32) *float32           { return &f }
func pfloat64(f float64) *float64           { return &f }
func pstring(s string) *string              { return &s }
func pbool(b bool) *bool                    { return &b }
func pinterface(v interface{}) *interface{} { return &v }

func ppint(n int) **int       { p := pint(n); return &p }
func pppint(n int) ***int     { p := ppint(n); return &p }
func ppTypeI(v TypeI) **TypeI { p := &v; return &p }

func TestDecoder(t *testing.T) {
	fixtures := []struct {
		desc           string
		in             string
		regFuncs       []interface{}
		out            interface{}
		expected       interface{}
		expectedRecord []string
		inheader       []string
		header         []string
		unused         []int
		err            error
	}{
		{
			desc: "embedded type - no tag - conflicting float tag",
			in:   "string,int,float,bool\nstring,5,2.5,t",
			out:  &TypeA{},
			expected: &TypeA{
				Embedded1: Embedded1{},
				Embedded2: Embedded2{Bool: true},
				String:    "string",
				Int:       5,
			},
			unused:         []int{2},
			expectedRecord: []string{"string", "5", "2.5", "t"},
			header:         []string{"string", "int", "float", "bool"},
		},
		{
			desc: "embedded type - with tag",
			in: `string,json
string,"{""key"":""value""}"
`,
			out: &TypeB{},
			expected: &TypeB{
				Embedded3: Embedded3{"key": "value"},
				String:    "string",
			},
			expectedRecord: []string{"string", `{"key":"value"}`},
			header:         []string{"string", "json"},
		},
		{
			desc: "embedded pointer type - no tag - type with conflicting tag",
			in:   "string,float\nstring,2.5",
			out:  &TypeC{},
			expected: &TypeC{
				Embedded1: &Embedded1{Float: 2.5},
				String:    "string",
			},
			expectedRecord: []string{"string", "2.5"},
			header:         []string{"string", "float"},
		},
		{
			desc: "embedded pointer type - with tag ",
			in: `string,json
string,"{""key"":""value""}"
`,
			out: &TypeD{},
			expected: &TypeD{
				Embedded3: &Embedded3{"key": "value"},
				String:    "string",
			},
			expectedRecord: []string{"string", `{"key":"value"}`},
			header:         []string{"string", "json"},
		},
		{
			desc: "pointer types",
			in:   "string,int\nstring,10",
			out:  &TypeE{},
			expected: &TypeE{
				String: &PString,
				Int:    &Int,
			},
			expectedRecord: []string{"string", "10"},
			header:         []string{"string", "int"},
		},
		{
			desc: "basic types",
			in: "int,pint,int8,pint8,int16,pint16,int32,pint32,int64,pint64,uint," +
				"puint,uint8,puint8,uint16,puint16,uint32,puint32,uint64,puint64,float32," +
				"pfloat32,float64,pfloat64,string,pstring,bool,pbool,interface,pinterface,binary,pbinary\n" +
				"1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,true,true,true,1," +
				EncodedBinary + "," + EncodedBinaryLarge,
			out: &TypeF{},
			expected: &TypeF{
				Int:      1,
				Pint:     pint(2),
				Int8:     3,
				Pint8:    pint8(4),
				Int16:    5,
				Pint16:   pint16(6),
				Int32:    7,
				Pint32:   pint32(8),
				Int64:    9,
				Pint64:   pint64(10),
				UInt:     11,
				Puint:    puint(12),
				Uint8:    13,
				Puint8:   puint8(14),
				Uint16:   15,
				Puint16:  puint16(16),
				Uint32:   17,
				Puint32:  puint32(18),
				Uint64:   19,
				Puint64:  puint64(20),
				Float32:  21,
				Pfloat32: pfloat32(22),
				Float64:  23,
				Pfloat64: pfloat64(24),
				String:   "25",
				PString:  pstring("26"),
				Bool:     true,
				Pbool:    pbool(true),
				V:        "true",
				Pv:       pinterface("1"),
				Binary:   Binary,
				PBinary:  &BinaryLarge,
			},
			expectedRecord: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12",
				"13", "14", "15", "16", "17", "18", "19", "20", "21", "22", "23", "24", "25", "26",
				"true", "true", "true", "1", EncodedBinary, EncodedBinaryLarge},
			header: []string{"int",
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
			desc: "tags and unexported fields",
			in:   "String,int,Float64,unexported1,unexported2\nstring,10,2.5,1,1",
			out:  &TypeG{},
			expected: &TypeG{
				String: "string",
			},
			expectedRecord: []string{"string", "10", "2.5", "1", "1"},
			unused:         []int{1, 2, 3, 4},
			header:         []string{"String", "int", "Float64", "unexported1", "unexported2"},
		},
		{
			desc:           "omitempty tag",
			in:             "String,int\n,",
			out:            &TypeI{},
			expected:       &TypeI{},
			expectedRecord: []string{"", ""},
			header:         []string{"String", "int"},
		},
		{
			desc:           "empty struct",
			in:             "String\n1",
			out:            &struct{}{},
			expected:       &struct{}{},
			expectedRecord: []string{"1"},
			header:         []string{"String"},
			unused:         []int{0},
		},
		{
			desc: "decode value receiver unmarshalers",
			in:   "1,2,3\n1,2,3",
			out: &struct {
				ValueRecUnmarshaler    ValueRecUnmarshaler  `csv:"1"`
				PtrValueRecUnmarshaler *ValueRecUnmarshaler `csv:"2"`
				Iface                  interface{}          `csv:"3"`
			}{
				ValueRecUnmarshaler{new(string)},
				&ValueRecUnmarshaler{new(string)},
				ValueRecUnmarshaler{new(string)},
			},
			expected: &struct {
				ValueRecUnmarshaler    ValueRecUnmarshaler  `csv:"1"`
				PtrValueRecUnmarshaler *ValueRecUnmarshaler `csv:"2"`
				Iface                  interface{}          `csv:"3"`
			}{
				ValueRecUnmarshaler{pstring("1")},
				&ValueRecUnmarshaler{pstring("2")},
				ValueRecUnmarshaler{pstring("3")},
			},
			expectedRecord: []string{"1", "2", "3"},
			header:         []string{"1", "2", "3"},
		},
		{
			desc: "decode value receiver registered func",
			in:   "1,2,3,4\n1,2,3,4",
			out: &struct {
				ValueRecUnmarshaler    ValueRecUnmarshaler  `csv:"1"`
				PtrValueRecUnmarshaler *ValueRecUnmarshaler `csv:"2"`
				Iface                  interface{}          `csv:"3"`
				Iface2                 interface{}          `csv:"4"`
			}{
				ValueRecUnmarshaler{new(string)},
				&ValueRecUnmarshaler{new(string)},
				ValueRecUnmarshaler{new(string)},
				&ValueRecUnmarshaler{new(string)},
			},
			expected: &struct {
				ValueRecUnmarshaler    ValueRecUnmarshaler  `csv:"1"`
				PtrValueRecUnmarshaler *ValueRecUnmarshaler `csv:"2"`
				Iface                  interface{}          `csv:"3"`
				Iface2                 interface{}          `csv:"4"`
			}{
				ValueRecUnmarshaler{pstring("scan: 1")},
				&ValueRecUnmarshaler{pstring("scan: 2")},
				ValueRecUnmarshaler{pstring("scan: 3")},
				&ValueRecUnmarshaler{pstring("scan: 4")},
			},
			regFuncs: []interface{}{
				func(data []byte, v ValueRecUnmarshaler) error {
					return v.Scan(data)
				},
			},
			expectedRecord: []string{"1", "2", "3", "4"},
			header:         []string{"1", "2", "3", "4"},
		},
		{
			desc: "decode value receiver registered func - T is interface",
			in:   "1,2,3,4\n1,2,3,4",
			out: &struct {
				ValueRecUnmarshaler    ValueRecUnmarshaler  `csv:"1"`
				PtrValueRecUnmarshaler *ValueRecUnmarshaler `csv:"2"`
				Iface                  interface{}          `csv:"3"`
				Iface2                 interface{}          `csv:"4"`
			}{
				ValueRecUnmarshaler{new(string)},
				&ValueRecUnmarshaler{new(string)},
				ValueRecUnmarshaler{new(string)},
				&ValueRecUnmarshaler{new(string)},
			},
			expected: &struct {
				ValueRecUnmarshaler    ValueRecUnmarshaler  `csv:"1"`
				PtrValueRecUnmarshaler *ValueRecUnmarshaler `csv:"2"`
				Iface                  interface{}          `csv:"3"`
				Iface2                 interface{}          `csv:"4"`
			}{
				ValueRecUnmarshaler{pstring("scan: 1")},
				&ValueRecUnmarshaler{pstring("scan: 2")},
				ValueRecUnmarshaler{pstring("scan: 3")},
				&ValueRecUnmarshaler{pstring("scan: 4")},
			},
			regFuncs: []interface{}{
				func(data []byte, v interface{ Scan([]byte) error }) error {
					return v.Scan(data)
				},
			},
			expectedRecord: []string{"1", "2", "3", "4"},
			header:         []string{"1", "2", "3", "4"},
		},
		{
			desc: "decode value receiver textmarshaler",
			in:   "1,2,3,4\n1,2,3,4",
			out: &struct {
				ValueRecTextUnmarshaler    ValueRecTextUnmarshaler  `csv:"1"`
				PtrValueRecTextUnmarshaler *ValueRecTextUnmarshaler `csv:"2"`
				Iface                      interface{}              `csv:"3"`
				Iface2                     interface{}              `csv:"4"`
			}{
				ValueRecTextUnmarshaler{new(string)},
				&ValueRecTextUnmarshaler{new(string)},
				ValueRecTextUnmarshaler{new(string)},
				&ValueRecTextUnmarshaler{new(string)},
			},
			expected: &struct {
				ValueRecTextUnmarshaler    ValueRecTextUnmarshaler  `csv:"1"`
				PtrValueRecTextUnmarshaler *ValueRecTextUnmarshaler `csv:"2"`
				Iface                      interface{}              `csv:"3"`
				Iface2                     interface{}              `csv:"4"`
			}{
				ValueRecTextUnmarshaler{pstring("1")},
				&ValueRecTextUnmarshaler{pstring("2")},
				ValueRecTextUnmarshaler{pstring("3")},
				&ValueRecTextUnmarshaler{pstring("4")},
			},
			expectedRecord: []string{"1", "2", "3", "4"},
			header:         []string{"1", "2", "3", "4"},
		},

		{
			desc: "decode unmarshalers",
			in:   "csv,pcsv,text,ptext,csv-text,pcsv-text\nfield,field,field,field,field,field",
			out:  &Unmarshalers{},
			expected: &Unmarshalers{
				CSVUnmarshaler:      CSVUnmarshaler{"unmarshalCSV:field"},
				PCSVUnmarshaler:     &CSVUnmarshaler{"unmarshalCSV:field"},
				TextUnmarshaler:     TextUnmarshaler{"unmarshalText:field"},
				PTextUnmarshaler:    &TextUnmarshaler{"unmarshalText:field"},
				CSVTextUnmarshaler:  CSVTextUnmarshaler{"unmarshalCSV:field"},
				PCSVTextUnmarshaler: &CSVTextUnmarshaler{"unmarshalCSV:field"},
			},
			expectedRecord: []string{"field", "field", "field", "field", "field", "field"},
			header:         []string{"csv", "pcsv", "text", "ptext", "csv-text", "pcsv-text"},
		},
		{
			desc: "decode embedded tagged unmarshalers",
			in:   "csv,text,csv-text\nfield,field,field",
			out:  &EmbeddedUnmarshalers{},
			expected: &EmbeddedUnmarshalers{
				CSVUnmarshaler:     CSVUnmarshaler{"unmarshalCSV:field"},
				TextUnmarshaler:    TextUnmarshaler{"unmarshalText:field"},
				CSVTextUnmarshaler: CSVTextUnmarshaler{"unmarshalCSV:field"},
			},
			expectedRecord: []string{"field", "field", "field"},
			header:         []string{"csv", "text", "csv-text"},
		},
		{
			desc: "decode pointer embedded tagged unmarshalers",
			in:   "csv,text,csv-text\nfield,field,field",
			out:  &EmbeddedPtrUnmarshalers{},
			expected: &EmbeddedPtrUnmarshalers{
				CSVUnmarshaler:     &CSVUnmarshaler{"unmarshalCSV:field"},
				TextUnmarshaler:    &TextUnmarshaler{"unmarshalText:field"},
				CSVTextUnmarshaler: &CSVTextUnmarshaler{"unmarshalCSV:field"},
			},
			expectedRecord: []string{"field", "field", "field"},
			header:         []string{"csv", "text", "csv-text"},
		},
		{
			desc: "custom header",
			in:   "string,10",
			out:  &TypeI{},
			expected: &TypeI{
				String: "string",
				Int:    10,
			},
			expectedRecord: []string{"string", "10"},
			inheader:       []string{"String", "int"},
			header:         []string{"String", "int"},
		},
		{
			desc: "tag priority over field",
			in:   "Foo\n1",
			out:  &TagPriority{},
			expected: &TagPriority{
				Foo: 0,
				Bar: 1,
			},
			expectedRecord: []string{"1"},
			header:         []string{"Foo"},
		},
		{
			desc: "decode into unexported embedded field",
			in:   "foo,bar\n1,1",
			out:  &UnexportedEmbedded{},
			expected: &UnexportedEmbedded{
				embedded{
					Foo: 1,
					bar: 0,
				},
			},
			expectedRecord: []string{"1", "1"},
			header:         []string{"foo", "bar"},
			unused:         []int{1},
		},
		{
			desc: "decode into ptr unexported embedded field",
			in:   "foo,bar\n1,1",
			out:  &UnexportedEmbeddedPtr{},
			expected: &UnexportedEmbeddedPtr{
				&embedded{
					Foo: 1,
					bar: 0,
				},
			},
			expectedRecord: []string{"1", "1"},
			header:         []string{"foo", "bar"},
			unused:         []int{1},
			// this test will fail starting go1.10
			err: ptrUnexportedEmbeddedDecodeErr,
		},
		{
			desc: "embedded field conflict #1",
			in:   "X,Y\n1,2",
			out:  &Embedded5{},
			expected: &Embedded5{
				Embedded8: Embedded8{
					Embedded9: Embedded9{Y: 2},
				},
			},
			expectedRecord: []string{"1", "2"},
			header:         []string{"X", "Y"},
			unused:         []int{0},
		},
		{
			desc: "embedded field conflict #2",
			in:   "X,Y\n1,2",
			out:  &Embedded10{},
			expected: &Embedded10{
				Embedded13: Embedded13{
					Embedded8: Embedded8{
						Embedded9: Embedded9{Y: 2},
					},
				},
			},
			expectedRecord: []string{"1", "2"},
			header:         []string{"X", "Y"},
			unused:         []int{0},
		},
		{
			desc:           "circular reference",
			in:             "X,Y\n1,2",
			out:            &A{},
			expected:       &A{X: 1, B: B{Y: 2}},
			expectedRecord: []string{"1", "2"},
			header:         []string{"X", "Y"},
		},
		{
			desc:           "primitive type alias with Unmarshaler",
			in:             "enum\nfirst",
			out:            &EnumType{},
			expected:       &EnumType{Enum: EnumFirst},
			expectedRecord: []string{"first"},
			header:         []string{"enum"},
		},
		{
			desc:           "alias type",
			in:             "Float\n3.14",
			out:            &struct{ Float float64 }{},
			expected:       &struct{ Float float64 }{3.14},
			expectedRecord: []string{"3.14"},
			header:         []string{"Float"},
		},
		{
			desc:           "empty base64 string",
			in:             "Binary,foo\n,1\n",
			out:            &struct{ Binary []byte }{},
			expected:       &struct{ Binary []byte }{[]byte{}},
			expectedRecord: []string{"", "1"},
			header:         []string{"Binary", "foo"},
			unused:         []int{1},
		},
		{
			desc: "inline fields",
			in: "int,Bool,Uint8,float,prefix-STR,prefix-int,prefix-Bool,prefix-Uint8,prefix-float,top-string,STR\n" +
				"1,true,1,1,j2,2,true,2,2,top-level-str,STR",
			out: &Inline{},
			expected: &Inline{
				J1: TypeJ{
					Int:        "1",
					Float:      "1",
					Embedded16: Embedded16{Bool: true, Uint8: 1},
				},
				J2: TypeJ{
					String:     "j2",
					Int:        "2",
					Float:      "2",
					Embedded16: Embedded16{Bool: true, Uint8: 2},
				},
				String:  "top-level-str",
				String2: "STR",
			},
			expectedRecord: []string{"1", "true", "1", "1", "j2", "2", "true", "2", "2", "top-level-str", "STR"},
			header:         []string{"int", "Bool", "Uint8", "float", "prefix-STR", "prefix-int", "prefix-Bool", "prefix-Uint8", "prefix-float", "top-string", "STR"},
		},
		{
			desc: "inline chain",
			in:   "AS,AAA,AA,S,A\n1,11,34,2,22",
			out:  &Inline5{},
			expected: &Inline5{
				A: Inline2{
					S: "1",
					A: Inline3{
						Inline4: Inline4{A: "11"},
					},
				},
				B: Inline2{
					S: "2",
					B: Inline3{
						Inline4: Inline4{A: "22"},
					},
				},
			},
			unused:         []int{2},
			expectedRecord: []string{"1", "11", "34", "2", "22"},
			header:         []string{"AS", "AAA", "AA", "S", "A"},
		},
		{
			desc: "cyclic inline - no prefix",
			in:   "X\n1",
			out:  &Inline6{},
			expected: &Inline6{
				A: Inline7{
					A: nil,
					X: 1,
				},
			},
			expectedRecord: []string{"1"},
			header:         []string{"X"},
		},
		{
			desc: "inline visibility rules",
			in:   "AA\n1",
			out:  &Inline8{},
			expected: &Inline8{
				AA: 1,
			},
			expectedRecord: []string{"1"},
			header:         []string{"AA"},
		},
		{
			desc: "initialized interface",
			in:   "Int,Float,String,Bool,Unmarshaler,NilPtr\n10,3.14,string,true,lol,nil",
			out: &struct{ Int, Float, String, Bool, Unmarshaler, NilPtr interface{} }{
				Int:         int(0),
				Float:       float64(0),
				String:      "",
				Bool:        false,
				Unmarshaler: CSVUnmarshaler{},
				NilPtr:      (*int)(nil),
			},
			expected: &struct{ Int, Float, String, Bool, Unmarshaler, NilPtr interface{} }{
				Int:         "10",
				Float:       "3.14",
				String:      "string",
				Bool:        "true",
				Unmarshaler: "lol",
				NilPtr:      "nil",
			},
			expectedRecord: []string{"10", "3.14", "string", "true", "lol", "nil"},
			header:         []string{"Int", "Float", "String", "Bool", "Unmarshaler", "NilPtr"},
		},
		{
			desc: "initialized ptr interface",
			in:   "Int,Float,String,Bool,Unmarshaler,DoublePtr\n10,3.14,string,true,lol,100",
			out: &struct{ Int, Float, String, Bool, Unmarshaler, DoublePtr interface{} }{
				Int:         pint(0),
				Float:       pfloat64(0),
				String:      pstring(""),
				Bool:        pbool(false),
				Unmarshaler: &CSVUnmarshaler{},
				DoublePtr:   ppint(0),
			},
			expected: &struct{ Int, Float, String, Bool, Unmarshaler, DoublePtr interface{} }{
				Int:    pint(10),
				Float:  pfloat64(3.14),
				String: pstring("string"),
				Bool:   pbool(true),
				Unmarshaler: &CSVUnmarshaler{
					String: "unmarshalCSV:lol",
				},
				DoublePtr: ppint(100),
			},
			expectedRecord: []string{"10", "3.14", "string", "true", "lol", "100"},
			header:         []string{"Int", "Float", "String", "Bool", "Unmarshaler", "DoublePtr"},
		},
		{
			desc: "initialized ptr interface fields",
			in:   "Int,Float,String,Bool,Unmarshaler\n10,3.14,string,true,lol",
			out: &struct{ Int, Float, String, Bool, Unmarshaler *interface{} }{
				Int:         pinterface(int(0)),
				Float:       pinterface(float64(0)),
				String:      pinterface(""),
				Bool:        pinterface(false),
				Unmarshaler: pinterface(CSVUnmarshaler{}),
			},
			expected: &struct{ Int, Float, String, Bool, Unmarshaler *interface{} }{
				Int:         pinterface("10"),
				Float:       pinterface("3.14"),
				String:      pinterface("string"),
				Bool:        pinterface("true"),
				Unmarshaler: pinterface("lol"),
			},
			expectedRecord: []string{"10", "3.14", "string", "true", "lol"},
			header:         []string{"Int", "Float", "String", "Bool", "Unmarshaler"},
		},
		{
			desc: "initialized ptr interface fields to ptr values",
			in:   "Int,Float,String,Bool,Unmarshaler,DoublePtr\n10,3.14,string,true,lol,30",
			out: &struct{ Int, Float, String, Bool, Unmarshaler, DoublePtr *interface{} }{
				Int:         pinterface(pint(0)),
				Float:       pinterface(pfloat64(0)),
				String:      pinterface(pstring("")),
				Bool:        pinterface(pbool(false)),
				Unmarshaler: pinterface(&CSVUnmarshaler{}),
				DoublePtr:   pinterface(ppint(0)),
			},
			expected: &struct{ Int, Float, String, Bool, Unmarshaler, DoublePtr *interface{} }{
				Int:    pinterface(pint(10)),
				Float:  pinterface(pfloat64(3.14)),
				String: pinterface(pstring("string")),
				Bool:   pinterface(pbool(true)),
				Unmarshaler: pinterface(&CSVUnmarshaler{
					String: "unmarshalCSV:lol",
				}),
				DoublePtr: pinterface(ppint(30)),
			},
			expectedRecord: []string{"10", "3.14", "string", "true", "lol", "30"},
			header:         []string{"Int", "Float", "String", "Bool", "Unmarshaler", "DoublePtr"},
		},
		{
			desc: "nil slice of structs",
			in:   "String,int\nfirst,1\nsecond,2",
			out:  &TypeISlice,
			expected: &[]TypeI{
				{String: "first", Int: 1},
				{String: "second", Int: 2},
			},
			expectedRecord: nil,
			header:         []string{"String", "int"},
		},
		{
			desc: "slice of structs",
			in:   "String,int\nfirst,1\nsecond,2",
			out:  &[]TypeI{},
			expected: &[]TypeI{
				{String: "first", Int: 1},
				{String: "second", Int: 2},
			},
			expectedRecord: nil,
			header:         []string{"String", "int"},
		},
		{
			desc: "slice of structs - pre-allocated",
			in:   "String,int\nfirst,1\nsecond,2",
			out:  &[]TypeI{0: {Int: 200}, 1024: {Int: 100}},
			expected: &[]TypeI{
				{String: "first", Int: 1},
				{String: "second", Int: 2},
			},
			expectedRecord: nil,
			header:         []string{"String", "int"},
		},
		{
			desc: "slice of pointer structs",
			in:   "String,int\nfirst,1\nsecond,2",
			out:  &[]*TypeI{},
			expected: &[]*TypeI{
				{String: "first", Int: 1},
				{String: "second", Int: 2},
			},
			expectedRecord: nil,
			header:         []string{"String", "int"},
		},
		{
			desc: "slice of double pointer structs",
			in:   "String,int\nfirst,1\nsecond,2",
			out:  &[]**TypeI{},
			expected: &[]**TypeI{
				ppTypeI(TypeI{String: "first", Int: 1}),
				ppTypeI(TypeI{String: "second", Int: 2}),
			},
			expectedRecord: nil,
			header:         []string{"String", "int"},
		},
		{
			desc: "invalid slice of interfaces",
			in:   "String,int\nfirst,1",
			out:  &[]interface{}{},
			err: &InvalidDecodeError{
				Type: reflect.TypeOf(&[]interface{}{}),
			},
		},
		{
			desc: "invalid slice of ints",
			in:   "String,int\nfirst,1",
			out:  &[]int{},
			err: &InvalidDecodeError{
				Type: reflect.TypeOf(&[]int{}),
			},
		},
		{
			desc: "invalid non pointer slice of ints",
			in:   "String,int\nfirst,1",
			out:  []int{},
			err: &InvalidDecodeError{
				Type: reflect.TypeOf([]int{}),
			},
		},
		{
			desc: "array of structs",
			in:   "String,int\nfirst,1\nsecond,2",
			out:  &[2]TypeI{},
			expected: &[2]TypeI{
				{String: "first", Int: 1},
				{String: "second", Int: 2},
			},
			expectedRecord: []string{"second", "2"},
			header:         []string{"String", "int"},
		},
		{
			desc: "array of pointer structs",
			in:   "String,int\nfirst,1\nsecond,2",
			out:  &[2]*TypeI{},
			expected: &[2]*TypeI{
				{String: "first", Int: 1},
				{String: "second", Int: 2},
			},
			expectedRecord: []string{"second", "2"},
			header:         []string{"String", "int"},
		},
		{
			desc: "array of double pointer structs",
			in:   "String,int\nfirst,1\nsecond,2",
			out:  &[2]**TypeI{},
			expected: &[2]**TypeI{
				ppTypeI(TypeI{String: "first", Int: 1}),
				ppTypeI(TypeI{String: "second", Int: 2}),
			},
			expectedRecord: []string{"second", "2"},
			header:         []string{"String", "int"},
		},
		{
			desc: "array of structs - bigger than the data set",
			in:   "String,int\nfirst,1\nsecond,2",
			out: &[4]TypeI{
				3: {String: "I should be zeroed out", Int: 1024},
			},
			expected: &[4]TypeI{
				0: {String: "first", Int: 1},
				1: {String: "second", Int: 2},
			},
			expectedRecord: nil,
			header:         []string{"String", "int"},
		},
		{
			desc: "array of structs - smaller than the data set",
			in:   "String,int\nfirst,1\nsecond,2",
			out:  &[1]TypeI{},
			expected: &[1]TypeI{
				0: {String: "first", Int: 1},
			},
			expectedRecord: []string{"first", "1"},
			header:         []string{"String", "int"},
		},
		{
			desc: "invalid array of interfaces",
			in:   "String,int\nfirst,1",
			out:  &[1]interface{}{},
			err: &InvalidDecodeError{
				Type: reflect.TypeOf(&[1]interface{}{}),
			},
		},
		{
			desc: "invalid array of ints",
			in:   "String,int\nfirst,1",
			out:  &[1]int{},
			err: &InvalidDecodeError{
				Type: reflect.TypeOf(&[1]int{}),
			},
		},
		{
			desc: "invalid non pointer array of ints",
			in:   "String,int\nfirst,1",
			out:  [1]int{},
			err: &InvalidDecodeError{
				Type: reflect.TypeOf([1]int{}),
			},
		},
		{
			desc: "unsupported type",
			in:   "string,int\ns,1",
			out:  &TypeWithInvalidField{},
			err: &UnsupportedTypeError{
				Type: reflect.TypeOf(TypeI{}),
			},
		},
		{
			desc: "unsupported double ptr type",
			in:   "A\n1",
			out: &struct {
				A **struct{}
			}{},
			err: &UnsupportedTypeError{
				Type: reflect.TypeOf(struct{}{}),
			},
		},
		{
			desc: "invalid int",
			in:   "Int,Foo\n,",
			out:  &struct{ Int int }{},
			err:  &UnmarshalTypeError{Value: "", Type: reflect.TypeOf(int(0))},
		},
		{
			desc: "int overflow",
			in:   "Int\n1024",
			out:  &struct{ Int int8 }{},
			err:  &UnmarshalTypeError{Value: "1024", Type: reflect.TypeOf(int8(0))},
		},
		{
			desc: "invalid int pointer",
			in:   "Int,Foo\nbar,",
			out:  &struct{ Int *int }{},
			err:  &UnmarshalTypeError{Value: "bar", Type: reflect.TypeOf(int(0))},
		},
		{
			desc: "invalid type pointer",
			in:   "Int,Foo\n,",
			out:  &struct{ Int *struct{} }{},
			err:  &UnsupportedTypeError{Type: reflect.TypeOf(struct{}{})},
		},
		{
			desc: "invalid uint",
			in:   "Uint,Foo\n,",
			out:  &struct{ Uint uint }{},
			err:  &UnmarshalTypeError{Value: "", Type: reflect.TypeOf(uint(0))},
		},
		{
			desc: "uint overflow",
			in:   "Uint\n1024",
			out:  &struct{ Uint uint8 }{},
			err:  &UnmarshalTypeError{Value: "1024", Type: reflect.TypeOf(uint8(0))},
		},
		{
			desc: "invalid uint pointer",
			in:   "Uint\na",
			out:  &struct{ Uint *uint }{},
			err:  &UnmarshalTypeError{Value: "a", Type: reflect.TypeOf(uint(0))},
		},
		{
			desc: "invalid float",
			in:   "Float,Foo\n,",
			out:  &struct{ Float float64 }{},
			err:  &UnmarshalTypeError{Value: "", Type: reflect.TypeOf(float64(0))},
		},
		{
			desc: "invalid float pointer",
			in:   "Float\na",
			out:  &struct{ Float *float64 }{},
			err:  &UnmarshalTypeError{Value: "a", Type: reflect.TypeOf(float64(0))},
		},
		{
			desc: "invalid bool",
			in:   "Bool,Foo\n,",
			out:  &struct{ Bool bool }{},
			err:  &UnmarshalTypeError{Value: "", Type: reflect.TypeOf(bool(false))},
		},
		{
			desc: "invalid interface",
			in:   "Interface,Foo\n,",
			out:  &struct{ Interface Unmarshaler }{},
			err:  &UnmarshalTypeError{Value: "", Type: csvUnmarshaler},
		},
		{
			desc: "invalid interface pointer",
			in:   "Interface,Foo\nbar,",
			out:  &struct{ Interface *Unmarshaler }{},
			err:  &UnmarshalTypeError{Value: "bar", Type: csvUnmarshaler},
		},
		{
			desc: "invalid field in embedded type",
			in:   "String,int\n1,1",
			out:  &struct{ InvalidType }{},
			err:  &UnsupportedTypeError{Type: reflect.TypeOf(struct{}{})},
		},
		{
			desc: "not a struct in decode",
			in:   "string,int\n1,1",
			out:  &Int,
			err:  &InvalidDecodeError{Type: reflect.TypeOf(&Int)},
		},
		{
			desc: "not a struct in decode - non ptr",
			in:   "string,int\n1,1",
			out:  Int,
			err:  &InvalidDecodeError{Type: reflect.TypeOf(Int)},
		},
		{
			desc: "invalid base64 string",
			in:   "Binary\n1",
			out:  &struct{ Binary []byte }{},
			err:  base64.CorruptInputError(0),
		},
		{
			desc: "invalid int under interface value",
			in:   "Int,Foo\n,",
			out:  &struct{ Int interface{} }{Int: pint(0)},
			err:  &UnmarshalTypeError{Value: "", Type: reflect.TypeOf(int(0))},
		},
		{
			desc: "unsupported type under interface value",
			in:   "Invalid\n1",
			out:  &struct{ Invalid interface{} }{Invalid: &InvalidType{}},
			err:  &UnsupportedTypeError{Type: reflect.TypeOf(InvalidType{})},
		},
		{
			desc: "no panic on embedded pointer fields with blank value",
			in:   "X,Y\n,",
			out:  &Embedded17{},
			expected: &Embedded17{
				Embedded18: &Embedded18{},
			},
			expectedRecord: []string{"", ""},
			header:         []string{"X", "Y"},
		},
		{
			desc: "set blank values to nil on pointers",
			in:   "X,Y\n1,",
			out: &Embedded17{
				Embedded18: &Embedded18{
					X: pfloat64(10),
					Y: pfloat64(20),
				},
			},
			expected: &Embedded17{
				Embedded18: &Embedded18{
					X: pfloat64(1),
					Y: nil,
				},
			},
			expectedRecord: []string{"1", ""},
			header:         []string{"X", "Y"},
		},
		{
			desc: "no panic on embedded pointer fields with blank value 2",
			in:   "X,Y\n1,",
			out:  &Embedded17{},
			expected: &Embedded17{
				Embedded18: &Embedded18{X: pfloat64(1)},
			},
			expectedRecord: []string{"1", ""},
			header:         []string{"X", "Y"},
		},
		{
			desc: "fails on blank non float string with ptr embedded",
			in:   "string,float\n,",
			out:  &TypeC{},
			err:  &UnmarshalTypeError{Type: reflect.TypeOf(float64(0)), Value: ""},
		},
		{
			desc: "blank values on embedded pointers",
			in:   "String,Int\n,",
			out:  &TypeK{},
			expected: &TypeK{
				&TypeL{String: "", Int: 0},
			},
			expectedRecord: []string{"", ""},
			header:         []string{"String", "Int"},
		},
		{
			desc: "blank values on pointers decode to nil",
			in: "int,pint,int8,pint8,int16,pint16,int32,pint32,int64,pint64,uint," +
				"puint,uint8,puint8,uint16,puint16,uint32,puint32,uint64,puint64,float32," +
				"pfloat32,float64,pfloat64,string,pstring,bool,pbool,interface,pinterface,binary,pbinary\n" +
				"1,,3,,5,,7,,9,,11,,13,,15,,17,,19,,21,,23,,25,,true,,true,," +
				EncodedBinary + "," + "",
			out: &TypeF{
				Pint: pint(10),
			},
			expected: &TypeF{
				Int:      1,
				Pint:     nil,
				Int8:     3,
				Pint8:    nil,
				Int16:    5,
				Pint16:   nil,
				Int32:    7,
				Pint32:   nil,
				Int64:    9,
				Pint64:   nil,
				UInt:     11,
				Puint:    nil,
				Uint8:    13,
				Puint8:   nil,
				Uint16:   15,
				Puint16:  nil,
				Uint32:   17,
				Puint32:  nil,
				Uint64:   19,
				Puint64:  nil,
				Float32:  21,
				Pfloat32: nil,
				Float64:  23,
				Pfloat64: nil,
				String:   "25",
				PString:  nil,
				Bool:     true,
				Pbool:    nil,
				V:        "true",
				Pv:       nil,
				Binary:   Binary,
				PBinary:  nil,
			},
			expectedRecord: []string{"1", "", "3", "", "5", "", "7", "", "9", "", "11", "",
				"13", "", "15", "", "17", "", "19", "", "21", "", "23", "", "25", "",
				"true", "", "true", "", EncodedBinary, ""},
			header: []string{"int",
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
			desc: "registered func",
			in:   "Int,Pint,Iface,Piface\na,b,c,d",
			out: &struct {
				Int    int
				Pint   *int
				Iface  interface{}
				Piface *interface{}
			}{Iface: pint(10), Piface: pinterface(pint(10))},
			expected: &struct {
				Int    int
				Pint   *int
				Iface  interface{}
				Piface *interface{}
			}{
				Int:    10,
				Pint:   pint(11),
				Iface:  pint(12),
				Piface: pinterface(pint(13)),
			},
			regFuncs: []interface{}{
				func(data []byte, n *int) error {
					x, err := strconv.ParseInt(string(data), 16, 64)
					if err != nil {
						return err
					}
					*n = int(x)
					return nil
				},
			},
			expectedRecord: []string{"a", "b", "c", "d"},
			header:         []string{"Int", "Pint", "Iface", "Piface"},
		},
		{
			desc: "registered func - initialized interface ptr",
			in:   "Iface,Piface\na,b",
			out: &struct {
				Iface  interface{}
				Piface *interface{}
			}{Iface: 10, Piface: pinterface(10)},
			expected: &struct {
				Iface  interface{}
				Piface *interface{}
			}{
				Iface:  "a",
				Piface: pinterface("b"),
			},
			regFuncs: []interface{}{
				func(data []byte, n *int) error {
					x, err := strconv.ParseInt(string(data), 16, 64)
					if err != nil {
						return err
					}
					*n = int(x)
					return nil
				},
			},
			expectedRecord: []string{"a", "b"},
			header:         []string{"Iface", "Piface"},
		},
		{
			desc: "registered func - interfaces",
			in:   "Int,Pint,Iface,Piface,Scanner,PScanner\n10,20,30,40,50,60",
			out: &struct {
				Int      IntStruct
				Pint     *IntStruct
				Iface    interface{}
				Piface   *interface{}
				Scanner  fmt.Scanner
				PScanner *fmt.Scanner
			}{
				Iface:    &IntStruct{},
				Piface:   pinterface(&IntStruct{}),
				Scanner:  &IntStruct{},
				PScanner: &[]fmt.Scanner{&IntStruct{}}[0],
			},
			expected: &struct {
				Int      IntStruct
				Pint     *IntStruct
				Iface    interface{}
				Piface   *interface{}
				Scanner  fmt.Scanner
				PScanner *fmt.Scanner
			}{
				Int:      IntStruct{Value: 10},
				Pint:     &IntStruct{Value: 20},
				Iface:    &IntStruct{Value: 30},
				Piface:   pinterface(&IntStruct{Value: 40}),
				Scanner:  &IntStruct{Value: 50},
				PScanner: &[]fmt.Scanner{&IntStruct{Value: 60}}[0],
			},
			regFuncs: []interface{}{
				func(data []byte, scanner fmt.Scanner) error {
					_, err := fmt.Sscan(string(data), scanner)
					return err
				},
			},
			expectedRecord: []string{"10", "20", "30", "40", "50", "60"},
			header:         []string{"Int", "Pint", "Iface", "Piface", "Scanner", "PScanner"},
		},
		{
			desc: "registered func - invalid interface",
			in:   "Foo\n1",
			regFuncs: []interface{}{
				func(data []byte, scanner fmt.Scanner) error {
					_, err := fmt.Sscan(string(data), scanner)
					return err
				},
			},
			out: &struct{ Foo fmt.Scanner }{},
			err: &UnmarshalTypeError{Value: "1", Type: reflect.TypeOf((*fmt.Scanner)(nil)).Elem()},
		},
		{
			desc: "registered func - invalid *interface",
			in:   "Foo\n1",
			regFuncs: []interface{}{
				func(data []byte, scanner fmt.Scanner) error {
					_, err := fmt.Sscan(string(data), scanner)
					return err
				},
			},
			out: &struct{ Foo *fmt.Scanner }{},
			err: &UnmarshalTypeError{Value: "1", Type: reflect.TypeOf((*fmt.Scanner)(nil)).Elem()},
		},
		{
			desc: "registered func - non ptr interface",
			in:   "Foo\n1",
			regFuncs: []interface{}{
				func(data []byte, scanner fmt.Scanner) error {
					_, err := fmt.Sscan(string(data), scanner)
					return err
				},
			},
			out:            &struct{ Foo interface{} }{Foo: (fmt.Scanner)(nil)},
			expected:       &struct{ Foo interface{} }{Foo: "1"},
			expectedRecord: []string{"1"},
			header:         []string{"Foo"},
		},
		{
			desc: "registered func - ptr interface",
			in:   "Foo\n1",
			regFuncs: []interface{}{
				func(data []byte, scanner fmt.Scanner) error {
					_, err := fmt.Sscan(string(data), scanner)
					return err
				},
			},
			out:            &struct{ Foo interface{} }{Foo: (*fmt.Scanner)(nil)},
			expected:       &struct{ Foo interface{} }{Foo: "1"},
			expectedRecord: []string{"1"},
			header:         []string{"Foo"},
		},
	}

	for _, f := range fixtures {
		t.Run(f.desc, func(t *testing.T) {
			r, err := NewDecoder(newCSVReader(strings.NewReader(f.in)), f.inheader...)
			if err != nil {
				t.Fatal(err)
			}

			for _, fn := range f.regFuncs {
				r.Register(fn)
			}

			err = r.Decode(&f.out)
			if f.err != nil {
				if !checkErr(f.err, err) {
					t.Errorf("want err=%v; got %v", f.err, err)
				}
				return
			}

			if err != nil {
				t.Errorf("want err=nil; got %v", err)
			}

			if !reflect.DeepEqual(r.Record(), f.expectedRecord) {
				t.Errorf("want rec=%q; got %q", f.expectedRecord, r.Record())
			}

			if !reflect.DeepEqual(f.out, f.expected) {
				t.Errorf("want %#v; got %#v", f.expected, f.out)
			}

			if !reflect.DeepEqual(r.Unused(), f.unused) {
				t.Errorf("want unused=%v; got %v", f.unused, r.Unused())
			}

			if !reflect.DeepEqual(r.Header(), f.header) {
				t.Errorf("want header=%v; got %v", f.header, r.Header())
			}
		})
	}

	t.Run("decode with custom tag", func(t *testing.T) {
		type Type struct {
			String string `customtag:"string"`
			Int    int    `customtag:"int"`
		}

		dec, err := NewDecoder(NewReader([]string{"string", "10"}), "string", "int")
		if err != nil {
			t.Fatal(err)
		}
		dec.Tag = "customtag"

		var tt Type
		if err := dec.Decode(&tt); err != nil {
			t.Errorf("want err=nil; got %v", err)
		}

		expected := Type{"string", 10}
		if !reflect.DeepEqual(tt, expected) {
			t.Errorf("want tt=%v; got %v", expected, tt)
		}
	})

	t.Run("decode with disallow missing columns", func(t *testing.T) {
		type Type struct {
			String string
			Int    int
			Float  float64
		}

		t.Run("all present", func(t *testing.T) {
			dec, err := NewDecoder(NewReader(
				[]string{"String", "Int", "Float"},
				[]string{"lol", "1", "2.0"},
			))
			if err != nil {
				t.Fatal(err)
			}
			dec.DisallowMissingColumns = true

			var tt Type
			if err := dec.Decode(&tt); err != nil {
				t.Fatalf("expected err to be nil; got %v", err)
			}

			if expected := (Type{"lol", 1, 2}); !reflect.DeepEqual(tt, expected) {
				t.Errorf("want=%v; got %v", expected, tt)
			}
		})

		fixtures := []struct {
			desc        string
			recs        [][]string
			missingCols []string
			msg         string
		}{
			{
				desc: "one missing",
				recs: [][]string{
					{"String", "Int"},
					{"lol", "1"},
				},
				missingCols: []string{"Float"},
				msg:         `csvutil: missing columns: "Float"`,
			},
			{
				desc: "two missing",
				recs: [][]string{
					{"String"},
					{"lol"},
				},
				missingCols: []string{"Int", "Float"},
				msg:         `csvutil: missing columns: "Int", "Float"`,
			},
			{
				desc: "all missing",
				recs: [][]string{
					{"w00t"},
					{"lol"},
				},
				missingCols: []string{"String", "Int", "Float"},
				msg:         `csvutil: missing columns: "String", "Int", "Float"`,
			},
		}

		for _, f := range fixtures {
			t.Run(f.desc, func(t *testing.T) {
				dec, err := NewDecoder(NewReader(f.recs...))
				if err != nil {
					t.Fatal(err)
				}
				dec.DisallowMissingColumns = true

				var tt Type
				err = dec.Decode(&tt)

				if err == nil {
					t.Fatal("expected err != nil")
				}

				mcerr, ok := err.(*MissingColumnsError)
				if !ok {
					t.Fatalf("expected err to be of *MissingColumnErr; got %[1]T (%[1]v)", err)
				}

				if !reflect.DeepEqual(mcerr.Columns, f.missingCols) {
					t.Errorf("expected missing columns to be %v; got %v", f.missingCols, mcerr.Columns)
				}

				if err.Error() != f.msg {
					t.Errorf("expected err message to be %q; got %q", f.msg, err.Error())
				}
			})
		}
	})

	t.Run("invalid unmarshal tests", func(t *testing.T) {
		var fixtures = []struct {
			v        interface{}
			expected string
		}{
			{nil, "csvutil: Decode(nil)"},
			{nilIface, "csvutil: Decode(nil)"},
			{struct{}{}, "csvutil: Decode(non-pointer struct {})"},
			{int(1), "csvutil: Decode(non-pointer int)"},
			{[]int{}, "csvutil: Decode(non-pointer []int)"},
			{(*int)(nil), "csvutil: Decode(invalid type *int)"},
			{(*[]int)(nil), "csvutil: Decode(invalid type *[]int)"},
			{(*[]*int)(nil), "csvutil: Decode(invalid type *[]*int)"},
			{(*[1]*int)(nil), "csvutil: Decode(invalid type *[1]*int)"},
			{&nilIface, "csvutil: Decode(invalid type *interface {})"},
			{(*TypeA)(nil), "csvutil: Decode(nil *csvutil.TypeA)"},
		}

		for _, f := range fixtures {
			r, err := NewDecoder(newCSVReader(strings.NewReader("string\ns")))
			if err != nil {
				t.Fatal(err)
			}
			err = r.Decode(f.v)
			if err == nil {
				t.Errorf("Decode expecting error, got nil")
				continue
			}
			if got := err.Error(); got != f.expected {
				t.Errorf("want Decode=%q; got %q", f.expected, got)
			}
		}
	})

	t.Run("header and field length mismatch", func(t *testing.T) {
		type Foo struct {
			Col1 string `csv:"col1"`
			Col2 string `csv:"col2"`
		}
		data := []byte("1,1,1")
		r, err := NewDecoder(newCSVReader(bytes.NewReader(data)), "col1", "col2")
		if err != nil {
			t.Fatal(err)
		}

		var foo Foo
		if err := r.Decode(&foo); err != ErrFieldCount {
			t.Errorf("want err=%v; got %v", ErrFieldCount, err)
		}
	})

	t.Run("decode different types", func(t *testing.T) {
		data := []byte(`
String,Int,Float,Bool
s,1,3.14,true
s,1,3.14,true
s,1,3.14,true
`)

		type A struct {
			String string
			Foo    string
		}

		type B struct {
			Int int
			Foo int
		}

		type C struct {
			Bool   bool
			Float  float64
			Int    int
			String string
			Foo    int
		}

		dec, err := NewDecoder(newCSVReader(bytes.NewReader(data)))
		if err != nil {
			t.Errorf("want err=nil; got %v", err)
		}

		fixtures := []struct {
			out      interface{}
			expected interface{}
			unused   []int
		}{
			{
				out:      &A{},
				expected: &A{String: "s"},
				unused:   []int{1, 2, 3},
			},
			{
				out:      &B{},
				expected: &B{Int: 1},
				unused:   []int{0, 2, 3},
			},
			{
				out: &C{},
				expected: &C{
					Bool:   true,
					Float:  3.14,
					Int:    1,
					String: "s",
				},
			},
		}

		for _, f := range fixtures {
			if err := dec.Decode(f.out); err != nil {
				t.Errorf("want err=nil; got %v", err)
			}
			if !reflect.DeepEqual(f.out, f.expected) {
				t.Errorf("want %v; got %v", f.expected, f.out)
			}
			if !reflect.DeepEqual(dec.Unused(), f.unused) {
				t.Errorf("want %v; got %v", f.unused, dec.Unused())
			}
		}
	})

	t.Run("decode NaN", func(t *testing.T) {
		data := []byte("F1,F2,F3,F4,F5\nNaN,nan,NAN,nAn,NaN")
		dec, err := NewDecoder(newCSVReader(bytes.NewReader(data)))
		if err != nil {
			t.Fatalf("want err=nil; got %v", err)
		}

		v := struct {
			F1, F2, F3, F4 float64
			F5             Float // aliased type
		}{}
		if err := dec.Decode(&v); err != nil {
			t.Fatalf("want err=nil; got %v", err)
		}

		for _, f := range []float64{v.F1, v.F2, v.F3, v.F4, float64(v.F5)} {
			if !math.IsNaN(f) {
				t.Errorf("want f=NaN; got %v", f)
			}
		}
	})

	t.Run("map", func(t *testing.T) {
		t.Run("receives non-pointer and non-interface zero values", func(t *testing.T) {
			data := []byte("int,pint,int8,pint8,int16,pint16,int32,pint32,int64,pint64,uint," +
				"puint,uint8,puint8,uint16,puint16,uint32,puint32,uint64,puint64,float32," +
				"pfloat32,float64,pfloat64,string,pstring,bool,pbool,interface,pinterface,binary,pbinary\n" +
				"1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,true,true,true,1," +
				EncodedBinary + "," + EncodedBinaryLarge)

			dec, err := NewDecoder(newCSVReader(bytes.NewReader(data)))
			if err != nil {
				t.Fatalf("want err=nil; got %v", err)
			}

			var out TypeF
			var counter int
			m := func(field, col string, v interface{}) string {
				switch v.(type) {
				case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64,
					float32, float64, string, bool, []byte:
					counter++ // interface values are passed as strings.
				}
				return field
			}
			dec.Map = m

			if err := dec.Decode(&out); err != nil {
				t.Errorf("want err=nil; got %v", err)
			}
			if numField := reflect.TypeOf(out).NumField(); counter != numField {
				t.Errorf("expected counter=%d; got %d", numField, counter)
			}
		})

		t.Run("replaced value", func(t *testing.T) {
			m := func(field, col string, v interface{}) string {
				if _, ok := v.(float64); ok && field == "n/a" {
					return "NaN"
				}
				return field
			}

			data := []byte("F1,F2,F3\nn/a,n/a,n/a")
			dec, err := NewDecoder(newCSVReader(bytes.NewReader(data)))
			if err != nil {
				t.Fatalf("want err=nil; got %v", err)
			}
			dec.Map = m

			var out struct {
				F1 float64
				F2 *float64
				F3 **float64
			}
			if err := dec.Decode(&out); err != nil {
				t.Errorf("want err=nil; got %v", err)
			}
			if !math.IsNaN(out.F1) {
				t.Errorf("want F1 to be NaN but is %f", out.F1)
			}
			if out.F2 == nil {
				t.Error("want F2 to not be nil")
			}
			if !math.IsNaN(*out.F2) {
				t.Errorf("want F2 to be NaN but is %f", *out.F2)
			}
			if out.F3 == nil {
				t.Error("want F3 to not be nil")
			}
			if *out.F3 == nil {
				t.Error("want *F3 to not be nil")
			}
			if !math.IsNaN(**out.F3) {
				t.Errorf("want F3 to be NaN but is %f", **out.F3)
			}
		})

		t.Run("unmarshaler types", func(t *testing.T) {
			m := func(field, col string, v interface{}) string {
				if _, ok := v.(CSVUnmarshaler); ok && field == "" {
					return "csv_unmarshaler"
				}
				if _, ok := v.(TextUnmarshaler); ok && field == "" {
					return "text_unmarshaler"
				}
				return field
			}

			data := []byte("csv,pcsv,text,ptext\n,,,")
			dec, err := NewDecoder(newCSVReader(bytes.NewReader(data)))
			if err != nil {
				t.Fatalf("want err=nil; got %v", err)
			}
			dec.Map = m

			var out Unmarshalers
			if err := dec.Decode(&out); err != nil {
				t.Errorf("want err=nil; got %v", err)
			}

			expected := Unmarshalers{
				CSVUnmarshaler:   CSVUnmarshaler{String: "unmarshalCSV:csv_unmarshaler"},
				PCSVUnmarshaler:  nil,
				TextUnmarshaler:  TextUnmarshaler{String: "unmarshalText:text_unmarshaler"},
				PTextUnmarshaler: nil,
			}

			if !reflect.DeepEqual(out, expected) {
				t.Errorf("want out=%v; got %v", expected, out)
			}
		})

		t.Run("interface types", func(t *testing.T) {
			m := func(field, col string, v interface{}) string {
				if _, ok := v.(string); ok {
					return strings.ToUpper(field)
				}
				if _, ok := v.(int); ok {
					return "100"
				}
				t.Fatalf("expected v to be a string, was %T", v)
				return field
			}

			data := []byte("F1,F2,F3,F4,F5,F6\na,b,3,4,5,6")
			dec, err := NewDecoder(newCSVReader(bytes.NewReader(data)))
			if err != nil {
				t.Fatalf("want err=nil; got %v", err)
			}
			dec.Map = m

			var out = struct {
				F1 interface{}
				F2 *interface{}
				F3 interface{}
				F4 interface{}
				F5 interface{}
				F6 *interface{}
			}{
				F3: int(0), // initialize an interface with a different type
				F4: pint(0),
				F5: (*int)(nil),
				F6: pinterface(ppint(0)),
			}
			if err := dec.Decode(&out); err != nil {
				t.Errorf("want err=nil; got %v", err)
			}

			if out.F1 != "A" {
				t.Errorf("expected F1=A got: %v", out.F1)
			}
			if *out.F2 != "B" {
				t.Errorf("expected F2=B got: %v", *out.F2)
			}
			if out.F3 != "3" {
				t.Errorf("expected F3=\"3\" got: %v", out.F3)
			}

			f4, ok := out.F4.(*int)
			if !ok || f4 == nil {
				t.Error("expected F4 to be non nil int ptr")
				return
			}
			if *f4 != 100 {
				t.Errorf("expected F4=100 got: %v", f4)
			}

			if out.F5 != "5" {
				t.Errorf("expected F5=\"5\" got: %v", out.F5)
			}

			f6, ok := (*out.F6).(**int)
			if !ok || f4 == nil {
				t.Error("expected F6 to be non nil int ptr")
				return
			}
			if **f6 != 100 {
				t.Errorf("expected F4=100 got: %v", f6)
			}
		})

		t.Run("receives a proper column name", func(t *testing.T) {
			const val = "magic_column"
			m := func(field, col string, v interface{}) string {
				if col == "F2" {
					return val
				}
				return field
			}

			data := []byte("F1,F2\na,b")
			dec, err := NewDecoder(newCSVReader(bytes.NewReader(data)))
			if err != nil {
				t.Fatalf("want err=nil; got %v", err)
			}
			dec.Map = m

			var out = struct {
				F1 string
				F2 string
			}{}
			if err := dec.Decode(&out); err != nil {
				t.Errorf("want err=nil; got %v", err)
			}

			if out.F1 != "a" {
				t.Errorf("expected F1=a got: %v", out.F1)
			}
			if out.F2 != val {
				t.Errorf("expected F2=%s got: %v", val, out.F1)
			}
		})
	})

	t.Run("decoding into specific values", func(t *testing.T) {
		setup := func(t *testing.T) *Decoder {
			data := []byte("String,Int\na,1")
			dec, err := NewDecoder(newCSVReader(bytes.NewReader(data)))
			if err != nil {
				t.Fatalf("want err=nil; got %v", err)
			}
			return dec
		}

		t.Run("wrapped in interfaces", func(t *testing.T) {
			dec := setup(t)

			var out *TypeG
			var ii interface{} = &out
			var i interface{} = ii
			if err := dec.Decode(&i); err != nil {
				t.Errorf("want err=nil; got %v", err)
			}
			if out == nil {
				t.Fatal("want out to not be nil")
			}
			if expected := (TypeG{String: "a", Int: 1}); *out != expected {
				t.Errorf("want expected=%v; got %v", expected, *out)
			}
		})

		t.Run("wrapped in interfaces #2", func(t *testing.T) {
			dec := setup(t)

			var out *TypeG
			var ii interface{} = &out
			var i interface{} = &ii
			if err := dec.Decode(&i); err != nil {
				t.Errorf("want err=nil; got %v", err)
			}
			if out == nil {
				t.Fatal("want out to not be nil")
			}
			if expected := (TypeG{String: "a", Int: 1}); *out != expected {
				t.Errorf("want expected=%v; got %v", expected, *out)
			}
		})

		t.Run("wrapped in interfaces not ptr", func(t *testing.T) {
			dec := setup(t)

			var out *TypeG
			var ii interface{} = out
			var i interface{} = ii

			expected := &InvalidDecodeError{Type: reflect.TypeOf(&TypeG{})}
			if err := dec.Decode(i); !reflect.DeepEqual(err, expected) {
				t.Errorf("want err=%v; got %v", expected, err)
			}
		})

		t.Run("wrapped in interface non ptr value", func(t *testing.T) {
			dec := setup(t)

			var out TypeG
			var ii interface{} = &out
			var i interface{} = ii
			if err := dec.Decode(&i); err != nil {
				t.Errorf("want err=nil; got %v", err)
			}
			if expected := (TypeG{String: "a", Int: 1}); out != expected {
				t.Errorf("want expected=%v; got %v", expected, out)
			}
		})

		t.Run("interface to interface", func(t *testing.T) {
			dec := setup(t)

			var ii interface{}
			var i interface{} = &ii

			expected := &InvalidDecodeError{Type: reflect.TypeOf((*interface{})(nil))}
			if err := dec.Decode(&i); !reflect.DeepEqual(err, expected) {
				t.Errorf("want err=%v; got %v", expected, err)
			}
		})

		t.Run("interface to nil interface", func(t *testing.T) {
			dec := setup(t)

			var ii *interface{}
			var i interface{} = ii

			expected := &InvalidDecodeError{Type: reflect.TypeOf((*interface{})(nil))}
			if err := dec.Decode(&i); !reflect.DeepEqual(err, expected) {
				t.Errorf("want err=%v; got %v", expected, err)
			}
		})

		t.Run("nil ptr value", func(t *testing.T) {
			dec := setup(t)

			var out *TypeG
			expected := &InvalidDecodeError{Type: reflect.TypeOf(&TypeG{})}
			if err := dec.Decode(out); !reflect.DeepEqual(err, expected) {
				t.Errorf("want err=%v; got %v", expected, err)
			}
		})

		t.Run("nil ptr value ptr", func(t *testing.T) {
			dec := setup(t)

			var out *TypeG
			if err := dec.Decode(&out); err != nil {
				t.Errorf("want err=nil; got %v", err)
			}
			if out == nil {
				t.Fatal("want out to not be nil")
			}
			if expected := (TypeG{String: "a", Int: 1}); *out != expected {
				t.Errorf("want expected=%v; got %v", expected, *out)
			}
		})

		t.Run("nil double ptr value", func(t *testing.T) {
			dec := setup(t)

			var out **TypeG

			expected := &InvalidDecodeError{Type: reflect.TypeOf(out)}
			if err := dec.Decode(out); !reflect.DeepEqual(err, expected) {
				t.Errorf("want err=%v; got %v", expected, err)
			}
		})

		t.Run("nil double ptr value ptr", func(t *testing.T) {
			dec := setup(t)

			var out **TypeG
			if err := dec.Decode(&out); err != nil {
				t.Errorf("want err=nil; got %v", err)
			}
			if out == nil {
				t.Fatal("want out to not be nil")
			}
			if expected := (TypeG{String: "a", Int: 1}); **out != expected {
				t.Errorf("want expected=%v; got %v", expected, **out)
			}
		})

		t.Run("non ptr value ptr", func(t *testing.T) {
			dec := setup(t)

			var out TypeG
			if err := dec.Decode(&out); err != nil {
				t.Errorf("want err=nil; got %v", err)
			}
			if expected := (TypeG{String: "a", Int: 1}); out != expected {
				t.Errorf("want expected=%v; got %v", expected, out)
			}
		})
	})

	t.Run("decode slice", func(t *testing.T) {
		csvr := csv.NewReader(strings.NewReader("String,int\nfirst,1\nsecond,2"))
		dec, err := NewDecoder(csvr)
		if err != nil {
			t.Fatalf("want err == nil; got %v", err)
		}

		var data []TypeI
		if err := dec.Decode(&data); err != nil {
			t.Errorf("want err=nil; got %v", err)
		}

		if len(data) != 2 {
			t.Fatalf("want len=2; got %d", len(data))
		}

		if err := dec.Decode(&data); err != io.EOF {
			t.Errorf("want err=EOF; got %v", err)
		}
	})

	t.Run("decode slice - error", func(t *testing.T) {
		csvr := csv.NewReader(strings.NewReader("String,int\nfirst,1\nsecond,notint\nthird,3"))
		dec, err := NewDecoder(csvr)
		if err != nil {
			t.Fatalf("want err == nil; got %v", err)
		}

		var data []TypeI
		if err := dec.Decode(&data); err == nil {
			t.Errorf("want err!=nil; got %v", err)
		}

		if len(data) != 2 {
			t.Errorf("want len=2; got %d", len(data))
		}

		if data[1].String != "second" {
			t.Errorf("want String=second; got %s", data[1].String)
		}
		if data[1].Int != 0 {
			t.Errorf("want Int=0; got %d", data[1].Int)
		}
	})

	t.Run("decode array", func(t *testing.T) {
		csvr := csv.NewReader(strings.NewReader("String,int\nfirst,1\nsecond,2"))
		dec, err := NewDecoder(csvr)
		if err != nil {
			t.Fatalf("want err == nil; got %v", err)
		}

		var data [1]TypeI
		if err := dec.Decode(&data); err != nil {
			t.Errorf("want err=nil; got %v", err)
		}

		if expected := (TypeI{String: "first", Int: 1}); data[0] != expected {
			t.Errorf("want %v; got %v", expected, data[0])
		}

		if err := dec.Decode(&data); err != nil {
			t.Errorf("want err=nil; got %v", err)
		}

		if expected := (TypeI{String: "second", Int: 2}); data[0] != expected {
			t.Errorf("want %v; got %v", expected, data[0])
		}

		if err := dec.Decode(&data); err != io.EOF {
			t.Errorf("want err=EOF; got %v", err)
		}
	})

	t.Run("decode array - error", func(t *testing.T) {
		csvr := csv.NewReader(strings.NewReader("String,int\nfirst,1\nsecond,notint"))
		dec, err := NewDecoder(csvr)
		if err != nil {
			t.Fatalf("want err == nil; got %v", err)
		}

		var data [2]TypeI
		if err := dec.Decode(&data); err == nil {
			t.Errorf("want err!=nil; got %v", err)
		}

		if data[1].String != "second" {
			t.Errorf("want String=second; got %s", data[1].String)
		}
		if data[1].Int != 0 {
			t.Errorf("want Int=0; got %d", data[1].Int)
		}
	})

	t.Run("register panics", func(t *testing.T) {
		dec, err := NewDecoder(csv.NewReader(nil), "foo")
		if err != nil {
			panic(err)
		}

		fixtures := []struct {
			desc string
			arg  interface{}
		}{
			{
				desc: "not a func",
				arg:  1,
			},
			{
				desc: "nil",
				arg:  nil,
			},
			{
				desc: "T == empty interface",
				arg:  func([]byte, interface{}) error { return nil },
			},
			{
				desc: "first in not bytes",
				arg:  func(int, int) error { return nil },
			},
			{
				desc: "out not error",
				arg:  func([]byte, *int) int { return 0 },
			},
			{
				desc: "func with one in value",
				arg:  func(int) error { return nil },
			},
			{
				desc: "func with no returns",
				arg:  func([]byte, int) {},
			},
		}

		for _, f := range fixtures {
			t.Run(f.desc, func(t *testing.T) {
				var e interface{}
				func() {
					defer func() {
						e = recover()
					}()
					dec.Register(f.arg)
				}()

				if e == nil {
					t.Error("Register was supposed to panic but it didnt")
				}
				t.Log(e)
			})
		}

		t.Run("already registered", func(t *testing.T) {
			f := func([]byte, int) error { return nil }
			dec.Register(f)

			var e interface{}
			func() {
				defer func() {
					e = recover()
				}()
				dec.Register(f)
			}()

			if e == nil {
				t.Error("Register was supposed to panic but it didnt")
			}
			t.Log(e)
		})
	})

	t.Run("normalize header", func(t *testing.T) {
		csvr := csv.NewReader(strings.NewReader("STRING,INT\nfirst,1"))
		dec, err := NewDecoder(csvr)
		if err != nil {
			t.Fatalf("want err == nil; got %v", err)
		}

		if err := dec.NormalizeHeader(strings.ToLower); err != nil {
			t.Fatalf("want err=nil; got %v", err)
		}

		var data struct {
			String string `csv:"string"`
			Int    int    `csv:"int"`
		}
		if err := dec.Decode(&data); err != nil {
			t.Fatalf("want err=nil; got %v", err)
		}

		if data.String != "first" {
			t.Errorf("want String=first; got %s", data.String)
		}
		if data.Int != 1 {
			t.Errorf("want Int=1; got %d", data.Int)
		}
	})

	t.Run("normalize header - duplicate error", func(t *testing.T) {
		csvr := csv.NewReader(strings.NewReader("STRING,string\nfirst,1"))
		dec, err := NewDecoder(csvr)
		if err != nil {
			t.Fatalf("want err == nil; got %v", err)
		}

		if err := dec.NormalizeHeader(strings.ToLower); err == nil {
			t.Fatal("want err not to be nil")
		}
	})
}

func BenchmarkDecode(b *testing.B) {
	type A struct {
		A int     `csv:"a"`
		B float64 `csv:"b"`
		C string  `csv:"c"`
		D int64   `csv:"d"`
		E int8    `csv:"e"`
		F float32 `csv:"f"`
		G float32 `csv:"g"`
		H float32 `csv:"h"`
		I string  `csv:"i"`
		J int     `csv:"j"`
	}

	header := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	record := []string{"1", "2.5", "xD", "6", "7", "8", "9", "10", "lol", "10"}

	fixtures := []struct {
		desc string
		len  int
	}{
		{"10 field struct 1 record", 1},
		{"10 field struct 10 records", 10},
		{"10 field struct 100 records", 100},
		{"10 field struct 1000 records", 1000},
		{"10 field struct 10000 records", 10000},
	}

	for _, f := range fixtures {
		var records [][]string
		for i := 0; i < f.len; i++ {
			records = append(records, record)
		}

		b.Run(f.desc, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				dec, err := NewDecoder(NewReader(records...), header...)
				if err != nil {
					b.Fatal(err)
				}
				var a A
				b.StartTimer()

				for {
					if err := dec.Decode(&a); err == io.EOF {
						break
					} else if err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}

	b.Run("10 field struct first decode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			dec, err := NewDecoder(NewReader(record), header...)
			if err != nil {
				b.Fatal(err)
			}

			var a A
			b.StartTimer()

			if err := dec.Decode(&a); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("10 field struct second decode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			r, err := NewDecoder(NewReader(record, record), header...)
			if err != nil {
				b.Fatal(err)
			}

			var a A
			if err := r.Decode(&a); err != nil {
				b.Fatal(err)
			}
			a = A{}
			b.StartTimer()

			if err := r.Decode(&a); err != nil {
				b.Fatal(err)
			}
		}
	})
}

type reader struct {
	records [][]string
	i       int
}

func NewReader(records ...[]string) Reader {
	return &reader{records, 0}
}

func (r *reader) Read() ([]string, error) {
	if r.i >= len(r.records) {
		return nil, io.EOF
	}
	r.i++
	return r.records[r.i-1], nil
}
