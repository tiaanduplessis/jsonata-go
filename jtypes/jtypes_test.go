package jtypes

import (
	"errors"
	"reflect"
	"testing"
)

type testVariant struct{}

func (testVariant) ValidTypes() []reflect.Type { return []reflect.Type{reflect.TypeOf("")} }

type testCallable struct{}

func (testCallable) Name() string    { return "test" }
func (testCallable) ParamCount() int { return 1 }
func (testCallable) Call([]reflect.Value) (reflect.Value, error) {
	return reflect.ValueOf(true), nil
}

type pointerCallable struct{}

func (*pointerCallable) Name() string    { return "pointer" }
func (*pointerCallable) ParamCount() int { return 0 }
func (*pointerCallable) Call([]reflect.Value) (reflect.Value, error) {
	return reflect.Value{}, nil
}

type testConvertible struct{}

func (testConvertible) ConvertTo(target reflect.Type) (reflect.Value, bool) {
	if target == reflect.TypeOf("") {
		return reflect.ValueOf("converted"), true
	}
	return reflect.Value{}, false
}

var (
	_ Variant     = testVariant{}
	_ Callable    = testCallable{}
	_ Callable    = (*pointerCallable)(nil)
	_ Convertible = testConvertible{}
	_ Optional    = (*OptionalBool)(nil)
	_ Optional    = (*OptionalInt)(nil)
	_ Optional    = (*OptionalFloat64)(nil)
	_ Optional    = (*OptionalString)(nil)
	_ Optional    = (*OptionalInterface)(nil)
	_ Optional    = (*OptionalValue)(nil)
	_ Optional    = (*OptionalCallable)(nil)
)

func TestPublicTypeVariables(t *testing.T) {
	if TypeOptional.Kind() != reflect.Interface || TypeCallable.Kind() != reflect.Interface ||
		TypeConvertible.Kind() != reflect.Interface || TypeVariant.Kind() != reflect.Interface {
		t.Fatal("interface type variables have incorrect kinds")
	}
	if TypeValue != reflect.TypeOf(reflect.Value{}) || TypeInterface != reflect.TypeOf((*interface{})(nil)).Elem() {
		t.Fatal("reflect type variables have incorrect values")
	}
	if ErrUndefined == nil || ErrUndefined.Error() != "undefined" {
		t.Fatal("ErrUndefined is not initialized correctly")
	}
}

func TestResolvePredicatesAndConverters(t *testing.T) {
	var number any = int32(7)
	wrapped := reflect.ValueOf(&number)
	if got := Resolve(wrapped); !got.IsValid() || got.Int() != 7 {
		t.Fatalf("Resolve = %#v", got)
	}
	if !IsBool(reflect.ValueOf(true)) || !IsString(reflect.ValueOf("text")) || !IsNumber(reflect.ValueOf(uint16(2))) {
		t.Fatal("basic predicates rejected valid values")
	}
	if IsString(reflect.ValueOf([]byte("text"))) {
		t.Fatal("[]byte should not be classified as a string")
	}
	if got, ok := AsString(reflect.ValueOf([]byte("text"))); ok || got != "" {
		t.Fatalf("AsString([]byte) = %q, %v; want rejection", got, ok)
	}
	type namedString string
	type namedByte byte
	type namedBool bool
	type namedInt int
	if got, ok := AsString(reflect.ValueOf(namedString("named"))); !ok || got != "named" {
		t.Fatalf("AsString(named string) = %q, %v", got, ok)
	}
	if IsString(reflect.ValueOf(namedByte(1))) {
		t.Fatal("named byte was classified as a string")
	}
	if got, ok := AsBool(reflect.ValueOf(namedBool(true))); !ok || !got || !IsBool(reflect.ValueOf(namedBool(false))) {
		t.Fatal("named bool was not recognized")
	}
	if got, ok := AsNumber(reflect.ValueOf(namedInt(8))); !ok || got != 8 || !IsNumber(reflect.ValueOf(namedInt(0))) {
		t.Fatal("named integer was not recognized")
	}
	if got, ok := AsString(reflect.ValueOf(3)); ok || got != "" {
		t.Fatalf("AsString(number) = %q, %v", got, ok)
	}
	if got, ok := AsBool(reflect.ValueOf(true)); !ok || !got {
		t.Fatalf("AsBool = %v, %v", got, ok)
	}
	if got, ok := AsNumber(reflect.ValueOf(int16(-3))); !ok || got != -3 {
		t.Fatalf("AsNumber = %v, %v", got, ok)
	}
	if !IsArrayOf(reflect.ValueOf([]int{1, 2}), IsNumber) {
		t.Fatal("IsArrayOf rejected numeric slice")
	}
	if IsArrayOf(reflect.ValueOf([]any{1, "2"}), IsNumber) {
		t.Fatal("IsArrayOf accepted mixed slice")
	}
	if !IsMap(reflect.ValueOf(map[string]int{})) || !IsStruct(reflect.ValueOf(struct{}{})) {
		t.Fatal("map or struct predicate rejected valid value")
	}
	if _, ok := AsNumber(reflect.Value{}); ok {
		t.Fatal("invalid reflect.Value converted to a number")
	}
}

func TestOptionalValues(t *testing.T) {
	boolOption := NewOptionalBool(true)
	if !boolOption.IsSet() || !boolOption.Bool || boolOption.Type() != reflect.TypeOf(false) {
		t.Fatal("optional bool was not initialized")
	}
	intOption := NewOptionalInt(4)
	floatOption := NewOptionalFloat64(2.5)
	stringOption := NewOptionalString("value")
	if !intOption.IsSet() || intOption.Int != 4 || !floatOption.IsSet() || floatOption.Float64 != 2.5 ||
		!stringOption.IsSet() || stringOption.String != "value" {
		t.Fatal("scalar optional constructor lost its value")
	}
	var interfaceOption OptionalInterface
	if interfaceOption.IsSet() {
		t.Fatal("zero optional interface is set")
	}
	interfaceOption.Set(reflect.ValueOf(9))
	if !interfaceOption.IsSet() || interfaceOption.Interface != 9 {
		t.Fatal("optional interface did not store its value")
	}
	valueOption := NewOptionalValue(reflect.ValueOf("value"))
	if !valueOption.IsSet() || !valueOption.Value.IsValid() || valueOption.Value.String() != "value" {
		t.Fatal("optional reflect.Value was not initialized")
	}
	valueOption.Set(reflect.ValueOf(reflect.ValueOf(7)))
	if !valueOption.Value.IsValid() || valueOption.Value.Int() != 7 {
		t.Fatal("optional reflect.Value did not unwrap a reflected value")
	}
	callableOption := NewOptionalCallable(testCallable{})
	if !callableOption.IsSet() || callableOption.Callable == nil || callableOption.Type() != TypeCallable {
		t.Fatal("optional callable was not initialized")
	}
	var unset OptionalString
	if !panics(func() { unset.Set(reflect.ValueOf(4)) }) || !unset.IsSet() {
		t.Fatal("optional string did not panic and set state on incompatible value")
	}
	var invalidBool OptionalBool
	var invalidInt OptionalInt
	var invalidFloat OptionalFloat64
	var invalidValue OptionalValue
	if !panics(func() { invalidBool.Set(reflect.Value{}) }) || !invalidBool.IsSet() ||
		!panics(func() { invalidInt.Set(reflect.Value{}) }) || !invalidInt.IsSet() ||
		!panics(func() { invalidFloat.Set(reflect.Value{}) }) || !invalidFloat.IsSet() ||
		!panics(func() { invalidValue.Set(reflect.Value{}) }) || !invalidValue.IsSet() {
		t.Fatal("optional setters did not preserve set state on invalid values")
	}
	var invalidInterface OptionalInterface
	if !panics(func() { invalidInterface.Set(reflect.Value{}) }) || !invalidInterface.IsSet() {
		t.Fatal("optional interface did not panic and set state on invalid value")
	}
	callableOption.Callable = testCallable{}
	if !panics(func() { callableOption.Set(reflect.Value{}) }) || !callableOption.IsSet() || callableOption.Callable != nil {
		t.Fatal("optional callable retained a stale value after invalid Set")
	}
	if !panics(func() { NewOptionalInterface(nil) }) || !panics(func() { NewOptionalCallable(nil) }) {
		t.Fatal("nil optional constructor did not preserve v1.5.4 panic behavior")
	}
}

func TestCallableVariantAndConvertible(t *testing.T) {
	callable := testCallable{}
	if !IsCallable(reflect.ValueOf(callable)) {
		t.Fatal("value callable was not recognized")
	}
	if got, ok := AsCallable(reflect.ValueOf(callable)); !ok || got.Name() != "test" || got.ParamCount() != 1 {
		t.Fatal("value callable conversion failed")
	}
	value := pointerCallable{}
	if !IsCallable(reflect.ValueOf(value)) {
		t.Fatal("unaddressable pointer-method value was not recognized")
	}
	if _, ok := AsCallable(reflect.ValueOf(value)); ok {
		t.Fatal("AsCallable returned an unaddressable pointer-method value")
	}
	if !IsCallable(reflect.ValueOf(&value)) {
		t.Fatal("pointer callable was not recognized")
	}
	if got, ok := AsCallable(reflect.ValueOf(&value)); !ok || got.Name() != "pointer" {
		t.Fatal("pointer callable conversion failed")
	}
	variant := testVariant{}
	if got := variant.ValidTypes(); len(got) != 1 || got[0].Kind() != reflect.String {
		t.Fatal("variant type list is incorrect")
	}
	converted, ok := (testConvertible{}).ConvertTo(reflect.TypeOf(""))
	if !ok || converted.String() != "converted" {
		t.Fatal("convertible test value failed")
	}
}

func TestArgumentHandlers(t *testing.T) {
	if !ArgCountEquals(2)([]reflect.Value{reflect.ValueOf(1), reflect.ValueOf(2)}) ||
		ArgCountEquals(2)([]reflect.Value{reflect.ValueOf(1)}) {
		t.Fatal("ArgCountEquals returned an incorrect result")
	}
	arguments := []reflect.Value{reflect.ValueOf(1), reflect.Value{}}
	if !ArgUndefined(1)(arguments) || ArgUndefined(0)(arguments) || ArgUndefined(2)(arguments) || ArgUndefined(-1)(arguments) {
		t.Fatal("ArgUndefined returned an incorrect result")
	}
}

func TestErrUndefinedIsDistinct(t *testing.T) {
	other := errors.New("undefined")
	if errors.Is(other, ErrUndefined) || errors.Is(ErrUndefined, other) {
		t.Fatal("ErrUndefined unexpectedly aliases another error")
	}
}

func TestInvalidAndTypedNilValues(t *testing.T) {
	var nilCallable *pointerCallable
	if !IsCallable(reflect.ValueOf(nilCallable)) {
		t.Fatal("typed nil callable was not recognized")
	}
	if IsCallable(reflect.Value{}) || IsBool(reflect.Value{}) || IsString(reflect.Value{}) || IsNumber(reflect.Value{}) {
		t.Fatal("invalid reflect.Value predicates were incorrect")
	}
	resolved := Resolve(reflect.ValueOf(nilCallable))
	if !resolved.IsValid() || resolved.Kind() != reflect.Pointer || !resolved.IsNil() {
		t.Fatal("Resolve did not preserve typed nil pointer")
	}
}

func panics(fn func()) (didPanic bool) {
	defer func() {
		didPanic = recover() != nil
	}()
	fn()
	return false
}
