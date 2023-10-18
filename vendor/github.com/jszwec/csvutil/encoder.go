package csvutil

import (
	"reflect"
	"sort"
)

const defaultBufSize = 4096

type encField struct {
	field
	encodeFunc
}

type encCache struct {
	fields []encField
	buf    []byte
	index  []int
	record []string
}

func newEncCache(k typeKey, funcMap map[reflect.Type]marshalFunc, funcs []marshalFunc, header []string) (_ *encCache, err error) {
	fields := cachedFields(k)
	encFields := make([]encField, 0, len(fields))

	// if header is not empty, we are going to track columns in a set and we will
	// track which columns are covered by type fields.
	set := make(map[string]bool, len(header))
	for _, s := range header {
		set[s] = false
	}

	for _, f := range fields {
		if _, ok := set[f.name]; len(header) > 0 && !ok {
			continue
		}
		set[f.name] = true

		fn, err := encodeFn(f.baseType, true, funcMap, funcs)
		if err != nil {
			return nil, err
		}

		encFields = append(encFields, encField{
			field:      f,
			encodeFunc: fn,
		})
	}

	if len(header) > 0 {
		// look for columns that were defined in a header but are not present
		// in the provided data type. In case we find any, we will set it to
		// a no-op encoder that always produces an empty column.
		for k, b := range set {
			if b {
				continue
			}
			encFields = append(encFields, encField{
				field: field{
					name: k,
				},
				encodeFunc: nopEncode,
			})
		}

		sortEncFields(header, encFields)
	}

	return &encCache{
		fields: encFields,
		buf:    make([]byte, 0, defaultBufSize),
		index:  make([]int, len(encFields)),
		record: make([]string, len(encFields)),
	}, nil
}

// sortEncFields sorts the provided fields according to the given header.
// at this stage header expects to contain matching fields, so both slices
// are expected to be of the same length.
func sortEncFields(header []string, fields []encField) {
	set := make(map[string]int, len(header))
	for i, s := range header {
		set[s] = i
	}

	sort.Slice(fields, func(i, j int) bool {
		return set[fields[i].name] < set[fields[j].name]
	})
}

// Encoder writes structs CSV representations to the output stream.
type Encoder struct {
	// Tag defines which key in the struct field's tag to scan for names and
	// options (Default: 'csv').
	Tag string

	// If AutoHeader is true, a struct header is encoded during the first call
	// to Encode automatically (Default: true).
	AutoHeader bool

	w          Writer
	c          *encCache
	header     []string
	noHeader   bool
	typeKey    typeKey
	funcMap    map[reflect.Type]marshalFunc
	ifaceFuncs []marshalFunc
}

// NewEncoder returns a new encoder that writes to w.
func NewEncoder(w Writer) *Encoder {
	return &Encoder{
		w:          w,
		noHeader:   true,
		AutoHeader: true,
	}
}

// Register registers a custom encoding function for a concrete type or interface.
// The argument f must be of type:
//
//	func(T) ([]byte, error)
//
// T must be a concrete type such as Foo or *Foo, or interface that has at
// least one method.
//
// During encoding, fields are matched by the concrete type first. If match is not
// found then Encoder looks if field implements any of the registered interfaces
// in order they were registered.
//
// Register panics if:
//   - f does not match the right signature
//   - f is an empty interface
//   - f was already registered
//
// Register is based on the encoding/json proposal:
// https://github.com/golang/go/issues/5901.
//
// Deprecated: use MarshalFunc function with type parameter instead. The benefits
// are type safety and much better performance.
func (e *Encoder) Register(f any) {
	var (
		fn  = reflect.ValueOf(f)
		typ = fn.Type()
	)

	if typ.Kind() != reflect.Func ||
		typ.NumIn() != 1 || typ.NumOut() != 2 ||
		typ.Out(0) != _bytes || typ.Out(1) != _error {
		panic("csvutil: func must be of type func(T) ([]byte, error)")
	}

	var (
		argType = typ.In(0)

		isIface = argType.Kind() == reflect.Interface
		isPtr   = argType.Kind() == reflect.Pointer
	)

	if isIface && argType.NumMethod() == 0 {
		panic("csvutil: func argument type must not be an empty interface")
	}

	wrappedFn := marshalFunc{
		f: func(val any) ([]byte, error) {
			v := reflect.ValueOf(val)
			if !v.IsValid() && (isIface || isPtr) {
				v = reflect.Zero(argType)
			}

			out := fn.Call([]reflect.Value{v})
			err, _ := out[1].Interface().(error)
			return out[0].Bytes(), err
		},
		argType: typ.In(0),
	}

	if e.funcMap == nil {
		e.funcMap = make(map[reflect.Type]marshalFunc)
	}

	if _, ok := e.funcMap[argType]; ok {
		panic("csvutil: func " + typ.String() + " already registered")
	}

	e.funcMap[argType] = wrappedFn

	if isIface {
		e.ifaceFuncs = append(e.ifaceFuncs, wrappedFn)
	}
}

// SetHeader overrides the provided data type's default header. Fields are
// encoded in the order of the provided header. If a column specified in the
// header doesn't exist in the provided type, it will be encoded as an empty
// column. Fields that are not part of the provided header are ignored.
// Encoder can't guarantee the right order if the provided header contains
// duplicate column names.
//
// SetHeader must be called before EncodeHeader and/or Encode in order to take
// effect.
func (enc *Encoder) SetHeader(header []string) {
	cp := make([]string, len(header))
	copy(cp, header)
	enc.header = cp
}

// WithMarshalers sets the provided Marshalers for the encoder.
//
// WithMarshalers are based on the encoding/json proposal:
// https://github.com/golang/go/issues/5901.
func (enc *Encoder) WithMarshalers(m *Marshalers) {
	enc.funcMap = m.funcMap
	enc.ifaceFuncs = m.ifaceFuncs
}

// Encode writes the CSV encoding of v to the output stream. The provided
// argument v must be a struct, struct slice or struct array.
//
// Only the exported fields will be encoded.
//
// First call to Encode will write a header unless EncodeHeader was called first
// or AutoHeader is false. Header names can be customized by using tags
// ('csv' by default), otherwise original Field names are used.
//
// If header was provided through SetHeader then it overrides the provided data
// type's default header. Fields are encoded in the order of the provided header.
// If a column specified in the header doesn't exist in the provided type, it will
// be encoded as an empty column. Fields that are not part of the provided header
// are ignored. Encoder can't guarantee the right order if the provided header
// contains duplicate column names.
//
// Header and fields are written in the same order as struct fields are defined.
// Embedded struct's fields are treated as if they were part of the outer struct.
// Fields that are embedded types and that are tagged are treated like any
// other field, but they have to implement Marshaler or encoding.TextMarshaler
// interfaces.
//
// Marshaler interface has the priority over encoding.TextMarshaler.
//
// Tagged fields have the priority over non tagged fields with the same name.
//
// Following the Go visibility rules if there are multiple fields with the same
// name (tagged or not tagged) on the same level and choice between them is
// ambiguous, then all these fields will be ignored.
//
// Nil values will be encoded as empty strings. Same will happen if 'omitempty'
// tag is set, and the value is a default value like 0, false or nil interface.
//
// Bool types are encoded as 'true' or 'false'.
//
// Float types are encoded using strconv.FormatFloat with precision -1 and 'G'
// format. NaN values are encoded as 'NaN' string.
//
// Fields of type []byte are being encoded as base64-encoded strings.
//
// Fields can be excluded from encoding by using '-' tag option.
//
// Examples of struct tags:
//
//	// Field appears as 'myName' header in CSV encoding.
//	Field int `csv:"myName"`
//
//	// Field appears as 'Field' header in CSV encoding.
//	Field int
//
//	// Field appears as 'myName' header in CSV encoding and is an empty string
//	// if Field is 0.
//	Field int `csv:"myName,omitempty"`
//
//	// Field appears as 'Field' header in CSV encoding and is an empty string
//	// if Field is 0.
//	Field int `csv:",omitempty"`
//
//	// Encode ignores this field.
//	Field int `csv:"-"`
//
//	// Encode treats this field exactly as if it was an embedded field and adds
//	// "my_prefix_" to each field's name.
//	Field Struct `csv:"my_prefix_,inline"`
//
//	// Encode treats this field exactly as if it was an embedded field.
//	Field Struct `csv:",inline"`
//
// Fields with inline tags that have a non-empty prefix must not be cyclic
// structures. Passing such values to Encode will result in an infinite loop.
//
// Encode doesn't flush data. The caller is responsible for calling Flush() if
// the used Writer supports it.
func (e *Encoder) Encode(v any) error {
	return e.encode(reflect.ValueOf(v))
}

// EncodeHeader writes the CSV header of the provided struct value to the output
// stream. The provided argument v must be a struct value.
//
// The first Encode method call will not write header if EncodeHeader was called
// before it. This method can be called in cases when a data set could be
// empty, but header is desired.
//
// EncodeHeader is like Header function, but it works with the Encoder and writes
// directly to the output stream. Look at Header documentation for the exact
// header encoding rules.
func (e *Encoder) EncodeHeader(v any) error {
	typ, err := valueType(v)
	if err != nil {
		return err
	}
	return e.encodeHeader(typ)
}

func (e *Encoder) encode(v reflect.Value) error {
	val := walkValue(v)

	if !val.IsValid() {
		return &InvalidEncodeError{}
	}

	switch val.Kind() {
	case reflect.Struct:
		return e.encodeStruct(val)
	case reflect.Array, reflect.Slice:
		if walkType(val.Type().Elem()).Kind() != reflect.Struct {
			return &InvalidEncodeError{v.Type()}
		}
		return e.encodeArray(val)
	default:
		return &InvalidEncodeError{v.Type()}
	}
}

func (e *Encoder) encodeStruct(v reflect.Value) error {
	if e.AutoHeader && e.noHeader {
		if err := e.encodeHeader(v.Type()); err != nil {
			return err
		}
	}
	return e.marshal(v)
}

func (e *Encoder) encodeArray(v reflect.Value) error {
	l := v.Len()
	for i := 0; i < l; i++ {
		if err := e.encodeStruct(walkValue(v.Index(i))); err != nil {
			return err
		}
	}
	return nil
}

func (e *Encoder) encodeHeader(typ reflect.Type) error {
	fields, _, _, record, err := e.cache(typ)
	if err != nil {
		return err
	}

	for i, f := range fields {
		record[i] = f.name
	}

	if err := e.w.Write(record); err != nil {
		return err
	}

	e.noHeader = false
	return nil
}

func (e *Encoder) marshal(v reflect.Value) error {
	fields, buf, index, record, err := e.cache(v.Type())
	if err != nil {
		return err
	}

	for i, f := range fields {
		v := walkIndex(v, f.index)

		omitempty := f.tag.omitEmpty
		if v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
			// We should disable omitempty for pointer and interface values,
			// because if it's nil we will automatically encode it as an empty
			// string. However, the initialized pointer should not be affected,
			// even if it's a default value.
			omitempty = false
		}

		if !v.IsValid() {
			index[i] = 0
			continue
		}

		b, err := f.encodeFunc(buf, v, omitempty)
		if err != nil {
			return err
		}
		index[i], buf = len(b)-len(buf), b
	}

	out := string(buf)
	for i, n := range index {
		record[i], out = out[:n], out[n:]
	}
	e.c.buf = buf[:0]

	return e.w.Write(record)
}

func (e *Encoder) tag() string {
	if e.Tag == "" {
		return defaultTag
	}
	return e.Tag
}

func (e *Encoder) cache(typ reflect.Type) ([]encField, []byte, []int, []string, error) {
	if k := (typeKey{e.tag(), typ}); k != e.typeKey {
		c, err := newEncCache(k, e.funcMap, e.ifaceFuncs, e.header)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		e.c, e.typeKey = c, k
	}
	return e.c.fields, e.c.buf[:0], e.c.index, e.c.record, nil
}

// Marshalers stores custom unmarshal functions. Marshalers are immutable.
//
// Marshalers are based on the encoding/json proposal:
// https://github.com/golang/go/issues/5901.
type Marshalers struct {
	funcMap    map[reflect.Type]marshalFunc
	ifaceFuncs []marshalFunc
}

// NewMarshalers merges the provided Marshalers into one and returns it.
// If Marshalers contain duplicate function signatures, the one that was
// provided first wins.
func NewMarshalers(ms ...*Marshalers) *Marshalers {
	out := &Marshalers{
		funcMap: make(map[reflect.Type]marshalFunc),
	}

	for _, u := range ms {
		for k, v := range u.funcMap {
			if _, ok := out.funcMap[k]; ok {
				continue
			}
			out.funcMap[k] = v
		}
		out.ifaceFuncs = append(out.ifaceFuncs, u.ifaceFuncs...)
	}

	return out
}

// MarshalFunc stores the provided function in Marshalers and returns it.
//
// T must be a concrete type such as Foo or *Foo, or interface that has at
// least one method.
//
// During encoding, fields are matched by the concrete type first. If match is not
// found then Encoder looks if field implements any of the registered interfaces
// in order they were registered.
//
// UnmarshalFunc panics if T is an empty interface.
func MarshalFunc[T any](f func(T) ([]byte, error)) *Marshalers {
	var (
		v       = reflect.ValueOf(f)
		typ     = v.Type()
		argType = typ.In(0)
		isIface = argType.Kind() == reflect.Interface
	)

	if isIface && argType.NumMethod() == 0 {
		panic("csvutil: func argument type must not be an empty interface")
	}

	var zero T
	wrappedFn := marshalFunc{
		f: func(v any) ([]byte, error) {
			if !isIface {
				return f(v.(T))
			}

			if v == nil {
				return f(zero)
			}
			return f(v.(T))
		},
		argType: typ.In(0),
	}

	funcMap := map[reflect.Type]marshalFunc{
		argType: wrappedFn,
	}

	var ifaceFuncs []marshalFunc
	if isIface {
		ifaceFuncs = []marshalFunc{
			wrappedFn,
		}
	}

	return &Marshalers{
		funcMap:    funcMap,
		ifaceFuncs: ifaceFuncs,
	}
}

func walkIndex(v reflect.Value, index []int) reflect.Value {
	for _, i := range index {
		v = walkPtr(v)
		if !v.IsValid() {
			return reflect.Value{}
		}
		v = v.Field(i)
	}
	return v
}

func walkPtr(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	return v
}

func walkValue(v reflect.Value) reflect.Value {
	for {
		switch v.Kind() {
		case reflect.Ptr, reflect.Interface:
			v = v.Elem()
		default:
			return v
		}
	}
}

func walkType(typ reflect.Type) reflect.Type {
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	return typ
}

type marshalFunc struct {
	f       func(any) ([]byte, error)
	argType reflect.Type
}
