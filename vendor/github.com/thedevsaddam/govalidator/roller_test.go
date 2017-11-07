package govalidator

import (
	"testing"
)

type Earth struct {
	Human
	Name     string
	Liveable bool
	Planet   map[string]interface{}
}

type Human struct {
	Male
	Female
}

type Male struct {
	Name string
	Age  int
}

type Female struct {
	Name string
	Age  int
}

type deepLevel struct {
	Deep   string
	Levels map[string]string
}

type structWithTag struct {
	Name string `validate:"name"`
	Age  int    `validate:"age"`
}

var p = map[string]interface{}{
	"naam":  "Jane",
	"bois":  29,
	"white": true,
}
var dl = deepLevel{
	Deep: "So much deep",
	Levels: map[string]string{
		"level 1": "20 m",
		"level 2": "30 m",
		"level 3": "80 m",
		"level 4": "103 m",
	},
}
var planet = map[string]interface{}{
	"name":      "mars",
	"age":       1000,
	"red":       true,
	"deepLevel": dl,
	"p":         p,
}
var male = Male{"John", 33}
var female = Female{"Jane", 30}
var h = Human{
	male,
	female,
}
var e = Earth{
	h,
	"green earth",
	true,
	planet,
}

var m = make(map[string]interface{}, 0)

type structWithPointerToEmbeddedStruct struct {
	Male   *Male
	Female *Female
	Planet *map[string]interface{}
}

func init() {
	m["earth"] = e
	m["person"] = "John Doe"
	m["iface"] = map[string]string{"another_person": "does it change root!"}
	m["array"] = [5]int{1, 4, 5, 6, 7}
}

func TestRoller_push(t *testing.T) {
	r := roller{}
	r.setTagIdentifier("validate")
	r.setTagSeparator("|")
	r.start(male)
	if r.push("Male.Name", "set new name") != false {
		t.Error("push failed!")
	}
}

func TestRoller_Start(t *testing.T) {
	r := roller{}
	r.setTagIdentifier("validate")
	r.setTagSeparator("|")
	r.start(m)
	if len(r.getFlatMap()) != 20 {
		t.Error("Start failed!")
	}
}

func BenchmarkRoller_Start(b *testing.B) {
	r := roller{}
	r.setTagIdentifier("validate")
	r.setTagSeparator("|")
	for n := 0; n < b.N; n++ {
		r.start(m)
	}
}

func Test_Roller_Start_empty_map(t *testing.T) {
	r := roller{}
	m := make(map[string]interface{}, 0)
	r.setTagIdentifier("validate")
	r.setTagSeparator("|")
	r.start(m)
	if len(r.getFlatMap()) > 0 {
		t.Error("Failed to validate empty map")
	}
}

func TestRoller_traverseStructWithEmbeddedPointerStructAndMap(t *testing.T) {
	r := roller{}
	s := structWithPointerToEmbeddedStruct{
		&male,
		&female,
		&p,
	}
	r.setTagIdentifier("validate")
	r.setTagSeparator("|")
	r.start(s)
	if len(r.getFlatMap()) != 4 {
		t.Error("traverseStructWithEmbeddedPointerStructAndMap failed!")
	}
}

func TestRoller_traverseMapWithPointerStructAndMap(t *testing.T) {
	r := roller{}
	mapOfPointerVals := map[string]interface{}{
		"structField":        male,
		"structPointerField": &female,
		"mapPointerField":    &p,
	}

	r.setTagIdentifier("validate")
	r.setTagSeparator("|")
	r.start(mapOfPointerVals)
	if len(r.getFlatMap()) != 7 {
		t.Error("traverseMapWithPointerStructAndMap failed!")
	}
}

func TestRoller_StartPointerToStruct(t *testing.T) {
	r := roller{}
	r.setTagIdentifier("validate")
	r.setTagSeparator("|")
	r.start(&male)
	if len(r.getFlatMap()) != 2 {
		t.Error("StartPointerToStruct failed!")
	}
}

func TestRoller_StartMap(t *testing.T) {
	r := roller{}
	r.setTagIdentifier("validate")
	r.setTagSeparator("|")
	r.start(m)
	if len(r.getFlatMap()) != 20 {
		t.Error("StartMap failed!")
	}
}

func TestRoller_StartPointerToMap(t *testing.T) {
	r := roller{}
	r.setTagIdentifier("validate")
	r.setTagSeparator("|")
	r.start(&p)
	if len(r.getFlatMap()) != 3 {
		t.Error("StartPointerToMap failed!")
	}
}

func TestRoller_StartStruct(t *testing.T) {
	r := roller{}
	r.setTagIdentifier("validate")
	r.setTagSeparator("|")
	r.start(h)

	if len(r.getFlatMap()) != 4 {
		t.Error("StartStruct failed!")
	}
}

func TestRoller_StartStructWithTag(t *testing.T) {
	r := roller{}
	swTag := structWithTag{"John", 44}
	r.setTagIdentifier("validate")
	r.setTagSeparator("|")
	r.start(swTag)

	if len(r.getFlatMap()) != 2 {
		t.Error("StartStructWithTag failed!")
	}
}

func TestRoller_StartStructPointerWithTag(t *testing.T) {
	r := roller{}
	swTag := structWithTag{"John", 44}
	r.setTagIdentifier("validate")
	r.setTagSeparator("|")
	r.start(&swTag)

	if len(r.getFlatMap()) != 2 {
		t.Error("StartStructPointerWithTag failed!")
	}
}

func TestRoller_GetFlatVal(t *testing.T) {
	r := roller{}
	r.setTagIdentifier("validate")
	r.setTagSeparator("|")
	r.start(m)

	//check struct field with string
	name, _ := r.getFlatVal("Male.Name")
	if name != "John" {
		t.Error("GetFlatVal failed for struct string field!")
	}

	//check struct field with int
	age, _ := r.getFlatVal("Male.Age")
	if age != 33 {
		t.Error("GetFlatVal failed for struct int field!")
	}

	//check struct field with array
	intArrOf5, _ := r.getFlatVal("array")
	if len(intArrOf5.([5]int)) != 5 {
		t.Error("GetFlatVal failed for struct array of [5]int field!")
	}

	//check map key of string
	person, _ := r.getFlatVal("person")
	if person != "John Doe" {
		t.Error("GetFlatVal failed for map[string]string!")
	}

	//check not existed key
	_, ok := r.getFlatVal("not_existed_key")
	if ok {
		t.Error("GetFlatVal failed for not available key!")
	}
}

func TestRoller_PremitiveDataType(t *testing.T) {
	mStr := map[string]string{"oneStr": "hello", "twoStr": "Jane", "threeStr": "Doe"}
	mBool := map[string]bool{"oneBool": true, "twoBool": false, "threeBool": true}
	mInt := map[string]int{"oneInt": 1, "twoInt": 2, "threeInt": 3}
	mInt8 := map[string]int8{"oneInt8": 1, "twoInt8": 2, "threeInt8": 3}
	mInt16 := map[string]int16{"oneInt16": 1, "twoInt16": 2, "threeInt16": 3}
	mInt32 := map[string]int32{"oneInt32": 1, "twoInt32": 2, "threeInt32": 3}
	mInt64 := map[string]int64{"oneInt64": 1, "twoInt64": 2, "threeInt64": 3}
	mFloat32 := map[string]float32{"onefloat32": 1.09, "twofloat32": 20.87, "threefloat32": 11.3}
	mFloat64 := map[string]float64{"onefloat64": 10.88, "twofloat64": 92.09, "threefloat64": 3.90}
	mUintptr := map[string]uintptr{"oneUintptr": 1, "twoUintptr": 2, "threeUintptr": 3}
	mUint := map[string]uint{"oneUint": 1, "twoUint": 2, "threeUint": 3}
	mUint8 := map[string]uint8{"oneUint8": 1, "twoUint8": 2, "threeUint8": 3}
	mUint16 := map[string]uint16{"oneUint16": 1, "twoUint16": 2, "threeUint16": 3}
	mUint32 := map[string]uint32{"oneUint32": 1, "twoUint32": 2, "threeUint32": 3}
	mUint64 := map[string]uint64{"oneUint64": 1, "twoUint64": 2, "threeUint64": 3}
	mComplex := map[string]interface{}{
		"ptrToMapString":  &mStr,
		"ptrToMapBool":    &mBool,
		"ptrToMapInt":     &mInt,
		"ptrToMapInt8":    &mInt8,
		"ptrToMapInt16":   &mInt16,
		"ptrToMapInt32":   &mInt32,
		"ptrToMapInt64":   &mInt64,
		"ptrToMapfloat32": &mFloat32,
		"ptrToMapfloat64": &mFloat64,
		"ptrToMapUintptr": &mUintptr,
		"ptrToMapUint":    &mUint,
		"ptrToMapUint8":   &mUint8,
		"ptrToMapUint16":  &mUint16,
		"ptrToMapUint32":  &mUint32,
		"ptrToMapUint64":  &mUint64,
	}
	r := roller{}
	r.setTagIdentifier("validate")
	r.setTagSeparator("|")
	r.start(mComplex)
	itemsLen := len(mComplex) * 3
	if len(r.getFlatMap()) != itemsLen {
		t.Error("PremitiveDataType failed!")
	}
}

func TestRoller_sliceOfType(t *testing.T) {
	males := []Male{
		{Name: "John", Age: 29},
		{Name: "Jane", Age: 23},
		{Name: "Tom", Age: 10},
	}
	r := roller{}
	r.setTagIdentifier("validate")
	r.setTagSeparator("|")
	r.start(males)
	i, _ := r.getFlatVal("slice")
	if len(i.([]Male)) != len(males) {
		t.Error("slice failed!")
	}
}

func TestRoller_ptrSliceOfType(t *testing.T) {
	males := []Male{
		{Name: "John", Age: 29},
		{Name: "Jane", Age: 23},
		{Name: "Tom", Age: 10},
	}
	r := roller{}
	r.setTagIdentifier("validate")
	r.setTagSeparator("|")
	r.start(&males)
	i, _ := r.getFlatVal("slice")
	if len(i.([]Male)) != len(males) {
		t.Error("slice failed!")
	}
}

func TestRoller_MapWithPointerPremitives(t *testing.T) {
	type customType string
	var str string
	var varInt int
	var varInt8 int8
	var varInt16 int16
	var varInt32 int32
	var varInt64 int64
	var varFloat32 float32
	var varFloat64 float64
	var varUint uint
	var varUint8 uint8
	var varUint16 uint16
	var varUint32 uint32
	var varUint64 uint64
	var varUintptr uintptr
	var x customType = "custom"
	y := []string{"y", "z"}

	males := map[string]interface{}{
		"string":     &str,
		"int":        &varInt,
		"int8":       &varInt8,
		"int16":      &varInt16,
		"int32":      &varInt32,
		"int64":      &varInt64,
		"float32":    &varFloat32,
		"float64":    &varFloat64,
		"uint":       &varUint,
		"uint8":      &varUint8,
		"uint16":     &varUint16,
		"uint32":     &varUint32,
		"uint64":     &varUint64,
		"uintPtr":    &varUintptr,
		"customType": &x,
		"y":          &y,
	}
	r := roller{}
	r.setTagIdentifier("validate")
	r.setTagSeparator("|")
	r.start(males)

	val, _ := r.getFlatVal("customType")
	if *val.(*customType) != "custom" {
		t.Error("fetching custom type value failed!")
	}

	valY, _ := r.getFlatVal("y")
	if len(*valY.(*[]string)) != len(y) {
		t.Error("fetching pointer to struct value failed!")
	}

	if len(r.getFlatMap()) != len(males) {
		t.Error("MapWithPointerPremitives failed!")
	}
}
