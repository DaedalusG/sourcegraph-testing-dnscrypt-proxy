// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package impl

import (
	"fmt"
	"reflect"

	pref "google.golang.org/protobuf/reflect/protoreflect"
)

// unwrapper unwraps the value to the underlying value.
// This is implemented by List and Map.
type unwrapper interface {
	protoUnwrap() any
}

// A Converter coverts to/from Go reflect.Value types and protobuf protoreflect.Value types.
type Converter interface {
	// PBValueOf converts a reflect.Value to a protoreflect.Value.
	PBValueOf(reflect.Value) pref.Value

	// GoValueOf converts a protoreflect.Value to a reflect.Value.
	GoValueOf(pref.Value) reflect.Value

	// IsValidPB returns whether a protoreflect.Value is compatible with this type.
	IsValidPB(pref.Value) bool

	// IsValidGo returns whether a reflect.Value is compatible with this type.
	IsValidGo(reflect.Value) bool

	// New returns a new field value.
	// For scalars, it returns the default value of the field.
	// For composite types, it returns a new mutable value.
	New() pref.Value

	// Zero returns a new field value.
	// For scalars, it returns the default value of the field.
	// For composite types, it returns an immutable, empty value.
	Zero() pref.Value
}

// NewConverter matches a Go type with a protobuf field and returns a Converter
// that converts between the two. Enums must be a named int32 kind that
// implements protoreflect.Enum, and messages must be pointer to a named
// struct type that implements protoreflect.ProtoMessage.
//
// This matcher deliberately supports a wider range of Go types than what
// protoc-gen-go historically generated to be able to automatically wrap some
// v1 messages generated by other forks of protoc-gen-go.
func NewConverter(t reflect.Type, fd pref.FieldDescriptor) Converter {
	switch {
	case fd.IsList():
		return newListConverter(t, fd)
	case fd.IsMap():
		return newMapConverter(t, fd)
	default:
		return newSingularConverter(t, fd)
	}
	panic(fmt.Sprintf("invalid Go type %v for field %v", t, fd.FullName()))
}

var (
	boolType    = reflect.TypeOf(bool(false))
	int32Type   = reflect.TypeOf(int32(0))
	int64Type   = reflect.TypeOf(int64(0))
	uint32Type  = reflect.TypeOf(uint32(0))
	uint64Type  = reflect.TypeOf(uint64(0))
	float32Type = reflect.TypeOf(float32(0))
	float64Type = reflect.TypeOf(float64(0))
	stringType  = reflect.TypeOf(string(""))
	bytesType   = reflect.TypeOf([]byte(nil))
	byteType    = reflect.TypeOf(byte(0))
)

var (
	boolZero    = pref.ValueOfBool(false)
	int32Zero   = pref.ValueOfInt32(0)
	int64Zero   = pref.ValueOfInt64(0)
	uint32Zero  = pref.ValueOfUint32(0)
	uint64Zero  = pref.ValueOfUint64(0)
	float32Zero = pref.ValueOfFloat32(0)
	float64Zero = pref.ValueOfFloat64(0)
	stringZero  = pref.ValueOfString("")
	bytesZero   = pref.ValueOfBytes(nil)
)

func newSingularConverter(t reflect.Type, fd pref.FieldDescriptor) Converter {
	defVal := func(fd pref.FieldDescriptor, zero pref.Value) pref.Value {
		if fd.Cardinality() == pref.Repeated {
			// Default isn't defined for repeated fields.
			return zero
		}
		return fd.Default()
	}
	switch fd.Kind() {
	case pref.BoolKind:
		if t.Kind() == reflect.Bool {
			return &boolConverter{t, defVal(fd, boolZero)}
		}
	case pref.Int32Kind, pref.Sint32Kind, pref.Sfixed32Kind:
		if t.Kind() == reflect.Int32 {
			return &int32Converter{t, defVal(fd, int32Zero)}
		}
	case pref.Int64Kind, pref.Sint64Kind, pref.Sfixed64Kind:
		if t.Kind() == reflect.Int64 {
			return &int64Converter{t, defVal(fd, int64Zero)}
		}
	case pref.Uint32Kind, pref.Fixed32Kind:
		if t.Kind() == reflect.Uint32 {
			return &uint32Converter{t, defVal(fd, uint32Zero)}
		}
	case pref.Uint64Kind, pref.Fixed64Kind:
		if t.Kind() == reflect.Uint64 {
			return &uint64Converter{t, defVal(fd, uint64Zero)}
		}
	case pref.FloatKind:
		if t.Kind() == reflect.Float32 {
			return &float32Converter{t, defVal(fd, float32Zero)}
		}
	case pref.DoubleKind:
		if t.Kind() == reflect.Float64 {
			return &float64Converter{t, defVal(fd, float64Zero)}
		}
	case pref.StringKind:
		if t.Kind() == reflect.String || (t.Kind() == reflect.Slice && t.Elem() == byteType) {
			return &stringConverter{t, defVal(fd, stringZero)}
		}
	case pref.BytesKind:
		if t.Kind() == reflect.String || (t.Kind() == reflect.Slice && t.Elem() == byteType) {
			return &bytesConverter{t, defVal(fd, bytesZero)}
		}
	case pref.EnumKind:
		// Handle enums, which must be a named int32 type.
		if t.Kind() == reflect.Int32 {
			return newEnumConverter(t, fd)
		}
	case pref.MessageKind, pref.GroupKind:
		return newMessageConverter(t)
	}
	panic(fmt.Sprintf("invalid Go type %v for field %v", t, fd.FullName()))
}

type boolConverter struct {
	goType reflect.Type
	def    pref.Value
}

func (c *boolConverter) PBValueOf(v reflect.Value) pref.Value {
	if v.Type() != c.goType {
		panic(fmt.Sprintf("invalid type: got %v, want %v", v.Type(), c.goType))
	}
	return pref.ValueOfBool(v.Bool())
}
func (c *boolConverter) GoValueOf(v pref.Value) reflect.Value {
	return reflect.ValueOf(v.Bool()).Convert(c.goType)
}
func (c *boolConverter) IsValidPB(v pref.Value) bool {
	_, ok := v.Interface().(bool)
	return ok
}
func (c *boolConverter) IsValidGo(v reflect.Value) bool {
	return v.IsValid() && v.Type() == c.goType
}
func (c *boolConverter) New() pref.Value  { return c.def }
func (c *boolConverter) Zero() pref.Value { return c.def }

type int32Converter struct {
	goType reflect.Type
	def    pref.Value
}

func (c *int32Converter) PBValueOf(v reflect.Value) pref.Value {
	if v.Type() != c.goType {
		panic(fmt.Sprintf("invalid type: got %v, want %v", v.Type(), c.goType))
	}
	return pref.ValueOfInt32(int32(v.Int()))
}
func (c *int32Converter) GoValueOf(v pref.Value) reflect.Value {
	return reflect.ValueOf(int32(v.Int())).Convert(c.goType)
}
func (c *int32Converter) IsValidPB(v pref.Value) bool {
	_, ok := v.Interface().(int32)
	return ok
}
func (c *int32Converter) IsValidGo(v reflect.Value) bool {
	return v.IsValid() && v.Type() == c.goType
}
func (c *int32Converter) New() pref.Value  { return c.def }
func (c *int32Converter) Zero() pref.Value { return c.def }

type int64Converter struct {
	goType reflect.Type
	def    pref.Value
}

func (c *int64Converter) PBValueOf(v reflect.Value) pref.Value {
	if v.Type() != c.goType {
		panic(fmt.Sprintf("invalid type: got %v, want %v", v.Type(), c.goType))
	}
	return pref.ValueOfInt64(int64(v.Int()))
}
func (c *int64Converter) GoValueOf(v pref.Value) reflect.Value {
	return reflect.ValueOf(int64(v.Int())).Convert(c.goType)
}
func (c *int64Converter) IsValidPB(v pref.Value) bool {
	_, ok := v.Interface().(int64)
	return ok
}
func (c *int64Converter) IsValidGo(v reflect.Value) bool {
	return v.IsValid() && v.Type() == c.goType
}
func (c *int64Converter) New() pref.Value  { return c.def }
func (c *int64Converter) Zero() pref.Value { return c.def }

type uint32Converter struct {
	goType reflect.Type
	def    pref.Value
}

func (c *uint32Converter) PBValueOf(v reflect.Value) pref.Value {
	if v.Type() != c.goType {
		panic(fmt.Sprintf("invalid type: got %v, want %v", v.Type(), c.goType))
	}
	return pref.ValueOfUint32(uint32(v.Uint()))
}
func (c *uint32Converter) GoValueOf(v pref.Value) reflect.Value {
	return reflect.ValueOf(uint32(v.Uint())).Convert(c.goType)
}
func (c *uint32Converter) IsValidPB(v pref.Value) bool {
	_, ok := v.Interface().(uint32)
	return ok
}
func (c *uint32Converter) IsValidGo(v reflect.Value) bool {
	return v.IsValid() && v.Type() == c.goType
}
func (c *uint32Converter) New() pref.Value  { return c.def }
func (c *uint32Converter) Zero() pref.Value { return c.def }

type uint64Converter struct {
	goType reflect.Type
	def    pref.Value
}

func (c *uint64Converter) PBValueOf(v reflect.Value) pref.Value {
	if v.Type() != c.goType {
		panic(fmt.Sprintf("invalid type: got %v, want %v", v.Type(), c.goType))
	}
	return pref.ValueOfUint64(uint64(v.Uint()))
}
func (c *uint64Converter) GoValueOf(v pref.Value) reflect.Value {
	return reflect.ValueOf(uint64(v.Uint())).Convert(c.goType)
}
func (c *uint64Converter) IsValidPB(v pref.Value) bool {
	_, ok := v.Interface().(uint64)
	return ok
}
func (c *uint64Converter) IsValidGo(v reflect.Value) bool {
	return v.IsValid() && v.Type() == c.goType
}
func (c *uint64Converter) New() pref.Value  { return c.def }
func (c *uint64Converter) Zero() pref.Value { return c.def }

type float32Converter struct {
	goType reflect.Type
	def    pref.Value
}

func (c *float32Converter) PBValueOf(v reflect.Value) pref.Value {
	if v.Type() != c.goType {
		panic(fmt.Sprintf("invalid type: got %v, want %v", v.Type(), c.goType))
	}
	return pref.ValueOfFloat32(float32(v.Float()))
}
func (c *float32Converter) GoValueOf(v pref.Value) reflect.Value {
	return reflect.ValueOf(float32(v.Float())).Convert(c.goType)
}
func (c *float32Converter) IsValidPB(v pref.Value) bool {
	_, ok := v.Interface().(float32)
	return ok
}
func (c *float32Converter) IsValidGo(v reflect.Value) bool {
	return v.IsValid() && v.Type() == c.goType
}
func (c *float32Converter) New() pref.Value  { return c.def }
func (c *float32Converter) Zero() pref.Value { return c.def }

type float64Converter struct {
	goType reflect.Type
	def    pref.Value
}

func (c *float64Converter) PBValueOf(v reflect.Value) pref.Value {
	if v.Type() != c.goType {
		panic(fmt.Sprintf("invalid type: got %v, want %v", v.Type(), c.goType))
	}
	return pref.ValueOfFloat64(float64(v.Float()))
}
func (c *float64Converter) GoValueOf(v pref.Value) reflect.Value {
	return reflect.ValueOf(float64(v.Float())).Convert(c.goType)
}
func (c *float64Converter) IsValidPB(v pref.Value) bool {
	_, ok := v.Interface().(float64)
	return ok
}
func (c *float64Converter) IsValidGo(v reflect.Value) bool {
	return v.IsValid() && v.Type() == c.goType
}
func (c *float64Converter) New() pref.Value  { return c.def }
func (c *float64Converter) Zero() pref.Value { return c.def }

type stringConverter struct {
	goType reflect.Type
	def    pref.Value
}

func (c *stringConverter) PBValueOf(v reflect.Value) pref.Value {
	if v.Type() != c.goType {
		panic(fmt.Sprintf("invalid type: got %v, want %v", v.Type(), c.goType))
	}
	return pref.ValueOfString(v.Convert(stringType).String())
}
func (c *stringConverter) GoValueOf(v pref.Value) reflect.Value {
	// pref.Value.String never panics, so we go through an interface
	// conversion here to check the type.
	s := v.Interface().(string)
	if c.goType.Kind() == reflect.Slice && s == "" {
		return reflect.Zero(c.goType) // ensure empty string is []byte(nil)
	}
	return reflect.ValueOf(s).Convert(c.goType)
}
func (c *stringConverter) IsValidPB(v pref.Value) bool {
	_, ok := v.Interface().(string)
	return ok
}
func (c *stringConverter) IsValidGo(v reflect.Value) bool {
	return v.IsValid() && v.Type() == c.goType
}
func (c *stringConverter) New() pref.Value  { return c.def }
func (c *stringConverter) Zero() pref.Value { return c.def }

type bytesConverter struct {
	goType reflect.Type
	def    pref.Value
}

func (c *bytesConverter) PBValueOf(v reflect.Value) pref.Value {
	if v.Type() != c.goType {
		panic(fmt.Sprintf("invalid type: got %v, want %v", v.Type(), c.goType))
	}
	if c.goType.Kind() == reflect.String && v.Len() == 0 {
		return pref.ValueOfBytes(nil) // ensure empty string is []byte(nil)
	}
	return pref.ValueOfBytes(v.Convert(bytesType).Bytes())
}
func (c *bytesConverter) GoValueOf(v pref.Value) reflect.Value {
	return reflect.ValueOf(v.Bytes()).Convert(c.goType)
}
func (c *bytesConverter) IsValidPB(v pref.Value) bool {
	_, ok := v.Interface().([]byte)
	return ok
}
func (c *bytesConverter) IsValidGo(v reflect.Value) bool {
	return v.IsValid() && v.Type() == c.goType
}
func (c *bytesConverter) New() pref.Value  { return c.def }
func (c *bytesConverter) Zero() pref.Value { return c.def }

type enumConverter struct {
	goType reflect.Type
	def    pref.Value
}

func newEnumConverter(goType reflect.Type, fd pref.FieldDescriptor) Converter {
	var def pref.Value
	if fd.Cardinality() == pref.Repeated {
		def = pref.ValueOfEnum(fd.Enum().Values().Get(0).Number())
	} else {
		def = fd.Default()
	}
	return &enumConverter{goType, def}
}

func (c *enumConverter) PBValueOf(v reflect.Value) pref.Value {
	if v.Type() != c.goType {
		panic(fmt.Sprintf("invalid type: got %v, want %v", v.Type(), c.goType))
	}
	return pref.ValueOfEnum(pref.EnumNumber(v.Int()))
}

func (c *enumConverter) GoValueOf(v pref.Value) reflect.Value {
	return reflect.ValueOf(v.Enum()).Convert(c.goType)
}

func (c *enumConverter) IsValidPB(v pref.Value) bool {
	_, ok := v.Interface().(pref.EnumNumber)
	return ok
}

func (c *enumConverter) IsValidGo(v reflect.Value) bool {
	return v.IsValid() && v.Type() == c.goType
}

func (c *enumConverter) New() pref.Value {
	return c.def
}

func (c *enumConverter) Zero() pref.Value {
	return c.def
}

type messageConverter struct {
	goType reflect.Type
}

func newMessageConverter(goType reflect.Type) Converter {
	return &messageConverter{goType}
}

func (c *messageConverter) PBValueOf(v reflect.Value) pref.Value {
	if v.Type() != c.goType {
		panic(fmt.Sprintf("invalid type: got %v, want %v", v.Type(), c.goType))
	}
	if c.isNonPointer() {
		if v.CanAddr() {
			v = v.Addr() // T => *T
		} else {
			v = reflect.Zero(reflect.PtrTo(v.Type()))
		}
	}
	if m, ok := v.Interface().(pref.ProtoMessage); ok {
		return pref.ValueOfMessage(m.ProtoReflect())
	}
	return pref.ValueOfMessage(legacyWrapMessage(v))
}

func (c *messageConverter) GoValueOf(v pref.Value) reflect.Value {
	m := v.Message()
	var rv reflect.Value
	if u, ok := m.(unwrapper); ok {
		rv = reflect.ValueOf(u.protoUnwrap())
	} else {
		rv = reflect.ValueOf(m.Interface())
	}
	if c.isNonPointer() {
		if rv.Type() != reflect.PtrTo(c.goType) {
			panic(fmt.Sprintf("invalid type: got %v, want %v", rv.Type(), reflect.PtrTo(c.goType)))
		}
		if !rv.IsNil() {
			rv = rv.Elem() // *T => T
		} else {
			rv = reflect.Zero(rv.Type().Elem())
		}
	}
	if rv.Type() != c.goType {
		panic(fmt.Sprintf("invalid type: got %v, want %v", rv.Type(), c.goType))
	}
	return rv
}

func (c *messageConverter) IsValidPB(v pref.Value) bool {
	m := v.Message()
	var rv reflect.Value
	if u, ok := m.(unwrapper); ok {
		rv = reflect.ValueOf(u.protoUnwrap())
	} else {
		rv = reflect.ValueOf(m.Interface())
	}
	if c.isNonPointer() {
		return rv.Type() == reflect.PtrTo(c.goType)
	}
	return rv.Type() == c.goType
}

func (c *messageConverter) IsValidGo(v reflect.Value) bool {
	return v.IsValid() && v.Type() == c.goType
}

func (c *messageConverter) New() pref.Value {
	if c.isNonPointer() {
		return c.PBValueOf(reflect.New(c.goType).Elem())
	}
	return c.PBValueOf(reflect.New(c.goType.Elem()))
}

func (c *messageConverter) Zero() pref.Value {
	return c.PBValueOf(reflect.Zero(c.goType))
}

// isNonPointer reports whether the type is a non-pointer type.
// This never occurs for generated message types.
func (c *messageConverter) isNonPointer() bool {
	return c.goType.Kind() != reflect.Ptr
}
