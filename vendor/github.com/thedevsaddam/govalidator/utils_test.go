package govalidator

import (
	"reflect"
	"testing"
)

func Test_isContainRequiredField(t *testing.T) {
	if !isContainRequiredField([]string{"required", "email"}) {
		t.Error("isContainRequiredField failed!")
	}

	if isContainRequiredField([]string{"numeric", "min:5"}) {
		t.Error("isContainRequiredField failed!")
	}
}

func Benchmark_isContainRequiredField(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isContainRequiredField([]string{"required", "email"})
	}
}

type person struct{}

func (person) Details() string {
	return "John Doe"
}

func (person) Age(age string) string {
	return "Age: " + age
}

func Test_isRuleExist(t *testing.T) {
	if !isRuleExist("required") {
		t.Error("isRuleExist failed for valid rule")
	}
	if isRuleExist("not exist") {
		t.Error("isRuleExist failed for invalid rule")
	}
}

func Test_toString(t *testing.T) {
	var Int int
	Int = 100
	str := toString(Int)
	typ := reflect.ValueOf(str).Kind()
	if typ != reflect.String {
		t.Error("toString failed!")
	}
}

func Test_isEmpty(t *testing.T) {
	var Int int
	var Int8 int
	var Float32 float32
	var Str string
	var Slice []int
	list := map[string]interface{}{
		"_int":     Int,
		"_int8":    Int8,
		"_float32": Float32,
		"_str":     Str,
		"_slice":   Slice,
	}
	for k, v := range list {
		if !isEmpty(v) {
			t.Errorf("%v failed", k)
		}
	}
}
