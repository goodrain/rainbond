package govalidator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

func Test_AddCustomRule(t *testing.T) {
	AddCustomRule("__x__", func(f string, rule string, message string, v interface{}) error {
		if v.(string) != "xyz" {
			return fmt.Errorf("The %s field must be xyz", f)
		}
		return nil
	})
	if len(rulesFuncMap) <= 0 {
		t.Error("AddCustomRule failed to add new rule")
	}
}

func Test_AddCustomRule_panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("AddCustomRule failed to panic")
		}
	}()
	AddCustomRule("__x__", func(f string, rule string, message string, v interface{}) error {
		if v.(string) != "xyz" {
			return fmt.Errorf("The %s field must be xyz", f)
		}
		return nil
	})
}

func Test_validateExtraRules(t *testing.T) {
	errsBag := url.Values{}
	validateCustomRules("f_field", "__x__", "a", "", errsBag)
	if len(errsBag) != 1 {
		t.Error("validateExtraRules failed")
	}
}

//================================= rules =================================
func Test_Required(t *testing.T) {
	type tRequired struct {
		Str     string  `json:"_str"`
		Int     int     `json:"_int"`
		Int8    int8    `json:"_int8"`
		Int16   int16   `json:"_int16"`
		Int32   int32   `json:"_int32"`
		Int64   int64   `json:"_int64"`
		Uint    uint    `json:"_uint"`
		Uint8   uint8   `json:"_uint8"`
		Uint16  uint16  `json:"_uint16"`
		Uint32  uint32  `json:"_uint32"`
		Uint64  uint64  `json:"_uint64"`
		Uintptr uintptr `json:"_uintptr"`
		Flaot32 float32 `json:"_float32"`
		Flaot64 float64 `json:"_float64"`
	}

	rules := MapData{
		"_str":     []string{"required"},
		"_int":     []string{"required"},
		"_int8":    []string{"required"},
		"_int16":   []string{"required"},
		"_int32":   []string{"required"},
		"_int64":   []string{"required"},
		"_uint":    []string{"required"},
		"_uint8":   []string{"required"},
		"_uint16":  []string{"required"},
		"_uint32":  []string{"required"},
		"_uint64":  []string{"required"},
		"_uintptr": []string{"required"},
		"_float32": []string{"required"},
		"_float64": []string{"required"},
	}

	postRequired := tRequired{}

	var trequired tRequired

	body, _ := json.Marshal(postRequired)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	messages := MapData{
		"_str": []string{"required:custom_message"},
	}

	opts := Options{
		Request:  req,
		Data:     &trequired,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 14 {
		t.Error("required validation failed!")
	}

	if validationErr.Get("_str") != "custom_message" {
		t.Error("required rule custom message failed")
	}
}

func Test_Regex(t *testing.T) {
	type tRegex struct {
		Name string `json:"name"`
	}

	postRegex := tRegex{Name: "john"}
	var tregex tRegex

	body, _ := json.Marshal(postRegex)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	messages := MapData{
		"name": []string{"regex:custom_message"},
	}

	rules := MapData{
		"name": []string{"regex:^[0-9]+$"},
	}

	opts := Options{
		Request:  req,
		Data:     &tregex,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 1 {
		t.Error("regex validation failed!")
	}

	if validationErr.Get("name") != "custom_message" {
		t.Error("regex rule custom message failed")
	}

}

func Test_Alpha(t *testing.T) {
	type user struct {
		Name string `json:"name"`
	}

	postUser := user{Name: "9080"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	messages := MapData{
		"name": []string{"alpha:custom_message"},
	}

	rules := MapData{
		"name": []string{"alpha"},
	}

	opts := Options{
		Request:  req,
		Data:     &userObj,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 1 {
		t.Error("alpha validation failed!")
	}

	if validationErr.Get("name") != "custom_message" {
		t.Error("alpha custom message failed!")
	}
}

func Test_AlphaDash(t *testing.T) {
	type user struct {
		Name string `json:"name"`
	}

	postUser := user{Name: "9090$"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	messages := MapData{
		"name": []string{"alpha_dash:custom_message"},
	}

	rules := MapData{
		"name": []string{"alpha_dash"},
	}

	opts := Options{
		Request:  req,
		Data:     &userObj,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 1 {
		t.Log(validationErr)
		t.Error("alpha_dash validation failed!")
	}

	if validationErr.Get("name") != "custom_message" {
		t.Error("alpha dash custom message failed!")
	}
}

func Test_AlphaNumeric(t *testing.T) {
	type user struct {
		Name string `json:"name"`
	}

	postUser := user{Name: "aE*Sb$"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"name": []string{"alpha_num"},
	}

	messages := MapData{
		"name": []string{"alpha_num:custom_message"},
	}

	opts := Options{
		Request:  req,
		Data:     &userObj,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 1 {
		t.Log(validationErr)
		t.Error("alpha_num validation failed!")
	}

	if validationErr.Get("name") != "custom_message" {
		t.Error("alpha num custom message failed!")
	}
}

func Test_Boolean(t *testing.T) {
	type Bools struct {
		BoolStr     string  `json:"boolStr"`
		BoolInt     int     `json:"boolInt"`
		BoolInt8    int8    `json:"boolInt8"`
		BoolInt16   int16   `json:"boolInt16"`
		BoolInt32   int32   `json:"boolInt32"`
		BoolInt64   int64   `json:"boolInt64"`
		BoolUint    uint    `json:"boolUint"`
		BoolUint8   uint8   `json:"boolUint8"`
		BoolUint16  uint16  `json:"boolUint16"`
		BoolUint32  uint32  `json:"boolUint32"`
		BoolUint64  uint64  `json:"boolUint64"`
		BoolUintptr uintptr `json:"boolUintptr"`
		Bool        bool    `json:"_bool"`
	}

	postBools := Bools{
		BoolStr:     "abc",
		BoolInt:     90,
		BoolInt8:    10,
		BoolInt16:   22,
		BoolInt32:   76,
		BoolInt64:   9,
		BoolUint:    5,
		BoolUint8:   9,
		BoolUint16:  9,
		BoolUint32:  9,
		BoolUint64:  8,
		BoolUintptr: 9,
	}
	var boolObj Bools

	body, _ := json.Marshal(postBools)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"boolStr":     []string{"bool"},
		"boolInt":     []string{"bool"},
		"boolInt8":    []string{"bool"},
		"boolInt16":   []string{"bool"},
		"boolInt32":   []string{"bool"},
		"boolInt64":   []string{"bool"},
		"boolUint":    []string{"bool"},
		"boolUint8":   []string{"bool"},
		"boolUint16":  []string{"bool"},
		"boolUint32":  []string{"bool"},
		"boolUint64":  []string{"bool"},
		"boolUintptr": []string{"bool"},
	}

	messages := MapData{
		"boolStr":  []string{"bool:custom_message"},
		"boolInt":  []string{"bool:custom_message"},
		"boolUint": []string{"bool:custom_message"},
	}

	opts := Options{
		Request:  req,
		Data:     &boolObj,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 12 {
		t.Error("bool validation failed!")
	}

	if validationErr.Get("boolStr") != "custom_message" ||
		validationErr.Get("boolInt") != "custom_message" ||
		validationErr.Get("boolUint") != "custom_message" {
		t.Error("bool custom message failed!")
	}
}

func Test_Between(t *testing.T) {
	type user struct {
		Str     string  `json:"str"`
		Int     int     `json:"_int"`
		Int8    int8    `json:"_int8"`
		Int16   int16   `json:"_int16"`
		Int32   int32   `json:"_int32"`
		Int64   int64   `json:"_int64"`
		Uint    uint    `json:"_uint"`
		Uint8   uint8   `json:"_uint8"`
		Uint16  uint16  `json:"_uint16"`
		Uint32  uint32  `json:"_uint32"`
		Uint64  uint64  `json:"_uint64"`
		Uintptr uintptr `json:"_uintptr"`
		Float32 float32 `json:"_float32"`
		Float64 float64 `json:"_float64"`
		Slice   []int   `json:"_slice"`
	}

	postUser := user{}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"str":      []string{"between:3,5"},
		"_int":     []string{"between:3,5"},
		"_int8":    []string{"between:3,5"},
		"_int16":   []string{"between:3,5"},
		"_int32":   []string{"between:3,5"},
		"_int64":   []string{"between:3,5"},
		"_uint":    []string{"between:3,5"},
		"_uint8":   []string{"between:3,5"},
		"_uint16":  []string{"between:3,5"},
		"_uint32":  []string{"between:3,5"},
		"_uint64":  []string{"between:3,5"},
		"_uintptr": []string{"between:3,5"},
		"_float32": []string{"between:3.5,5.9"},
		"_float64": []string{"between:3.3,6.2"},
		"_slice":   []string{"between:3,5"},
	}

	opts := Options{
		Request: req,
		Data:    &userObj,
		Rules:   rules,
	}

	vd := New(opts)
	vd.SetDefaultRequired(true)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 15 {
		t.Error("between validation failed!")
	}
}

func Test_CreditCard(t *testing.T) {
	type user struct {
		CreditCard string `json:"credit_card"`
	}

	postUser := user{CreditCard: "87080"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	messages := MapData{
		"credit_card": []string{"credit_card:custom_message"},
	}

	rules := MapData{
		"credit_card": []string{"credit_card"},
	}

	opts := Options{
		Request:  req,
		Data:     &userObj,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 1 {
		t.Error("credit card validation failed!")
	}

	if validationErr.Get("credit_card") != "custom_message" {
		t.Error("credit_card custom message failed!")
	}
}

func Test_Coordinate(t *testing.T) {
	type user struct {
		Coordinate string `json:"coordinate"`
	}

	postUser := user{Coordinate: "8080"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	messages := MapData{
		"coordinate": []string{"coordinate:custom_message"},
	}

	rules := MapData{
		"coordinate": []string{"coordinate"},
	}

	opts := Options{
		Request:  req,
		Data:     &userObj,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 1 {
		t.Error("coordinate validation failed!")
	}

	if validationErr.Get("coordinate") != "custom_message" {
		t.Error("coordinate custom message failed!")
	}
}

func Test_CSSColor(t *testing.T) {
	type user struct {
		Color string `json:"color"`
	}

	postUser := user{Color: "8080"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"color": []string{"css_color"},
	}

	messages := MapData{
		"color": []string{"css_color:custom_message"},
	}

	opts := Options{
		Request:  req,
		Data:     &userObj,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 1 {
		t.Error("CSS color validation failed!")
	}

	if validationErr.Get("color") != "custom_message" {
		t.Error("css_color custom message failed!")
	}
}

func Test_Digits(t *testing.T) {
	type user struct {
		Zip   string `json:"zip"`
		Level string `json:"level"`
	}

	postUser := user{Zip: "8322", Level: "10"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"zip":   []string{"digits:5"},
		"level": []string{"digits:1"},
	}

	opts := Options{
		Request: req,
		Data:    &userObj,
		Rules:   rules,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 2 {
		t.Error("Digits validation failed!")
	}
}

func Test_DigitsBetween(t *testing.T) {
	type user struct {
		Zip   string `json:"zip"`
		Level string `json:"level"`
	}

	postUser := user{Zip: "8322", Level: "10"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"zip":   []string{"digits_between:5,10"},
		"level": []string{"digits_between:5,10"},
	}

	messages := MapData{
		"zip":   []string{"digits_between:custom_message"},
		"level": []string{"digits_between:custom_message"},
	}

	opts := Options{
		Request:  req,
		Data:     &userObj,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 2 {
		t.Error("digits between validation failed!")
	}

	if validationErr.Get("zip") != "custom_message" ||
		validationErr.Get("level") != "custom_message" {
		t.Error("digits_between custom message failed!")
	}
}

func Test_DigitsBetweenPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Digits between failed to panic!")
		}
	}()
	type user struct {
		Zip   string `json:"zip"`
		Level string `json:"level"`
	}

	postUser := user{Zip: "8322", Level: "10"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"zip":   []string{"digits_between:5"},
		"level": []string{"digits_between:i,k"},
	}

	opts := Options{
		Request: req,
		Data:    &userObj,
		Rules:   rules,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 2 {
		t.Error("Digits between panic failed!")
	}
}

func Test_Date(t *testing.T) {
	type user struct {
		DOB         string `json:"dob"`
		JoiningDate string `json:"joining_date"`
	}

	postUser := user{DOB: "invalida date", JoiningDate: "10"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"dob":          []string{"date"},
		"joining_date": []string{"date:dd-mm-yyyy"},
	}

	opts := Options{
		Request: req,
		Data:    &userObj,
		Rules:   rules,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 2 {
		t.Log(validationErr)
		t.Error("Date validation failed!")
	}
}

func Test_Date_message(t *testing.T) {
	type user struct {
		DOB         string `json:"dob"`
		JoiningDate string `json:"joining_date"`
	}

	postUser := user{DOB: "invalida date", JoiningDate: "10"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"dob":          []string{"date"},
		"joining_date": []string{"date:dd-mm-yyyy"},
	}

	messages := MapData{
		"dob":          []string{"date:custom_message"},
		"joining_date": []string{"date:dd-mm-yyyy:custom_message"},
	}

	opts := Options{
		Request:  req,
		Data:     &userObj,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if validationErr.Get("dob") != "custom_message" {
		t.Error("Date custom message validation failed!")
	}
	if k := validationErr.Get("dob"); k != "custom_message" {
		t.Error("Date date:dd-mm-yyyy custom message validation failed!")
	}
}

func Test_Email(t *testing.T) {
	type user struct {
		Email string `json:"email"`
	}

	postUser := user{Email: "invalid email"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"email": []string{"email"},
	}

	opts := Options{
		Request: req,
		Data:    &userObj,
		Rules:   rules,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 1 {
		t.Log(validationErr)
		t.Error("Email validation failed!")
	}
}

func Test_Email_message(t *testing.T) {
	type user struct {
		Email string `json:"email"`
	}

	postUser := user{Email: "invalid email"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"email": []string{"email"},
	}

	messages := MapData{
		"email": []string{"email:custom_message"},
	}
	opts := Options{
		Request:  req,
		Data:     &userObj,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if validationErr.Get("email") != "custom_message" {
		t.Error("Email message validation failed!")
	}
}

func Test_Float(t *testing.T) {
	type user struct {
		CGPA string `json:"cgpa"`
	}

	postUser := user{CGPA: "invalid cgpa"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"cgpa": []string{"float"},
	}

	opts := Options{
		Request: req,
		Data:    &userObj,
		Rules:   rules,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 1 {
		t.Log(validationErr)
		t.Error("Float validation failed!")
	}
}

func Test_Float_message(t *testing.T) {
	type user struct {
		CGPA string `json:"cgpa"`
	}

	postUser := user{CGPA: "invalid cgpa"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"cgpa": []string{"float"},
	}

	messages := MapData{
		"cgpa": []string{"float:custom_message"},
	}

	opts := Options{
		Request:  req,
		Data:     &userObj,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if validationErr.Get("cgpa") != "custom_message" {
		t.Error("Float custom message failed!")
	}
}

func Test_IP(t *testing.T) {
	type user struct {
		IP string `json:"ip"`
	}

	postUser := user{IP: "invalid IP"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"ip": []string{"ip"},
	}

	opts := Options{
		Request: req,
		Data:    &userObj,
		Rules:   rules,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 1 {
		t.Log(validationErr)
		t.Error("IP validation failed!")
	}
}

func Test_IP_message(t *testing.T) {
	type user struct {
		IP string `json:"ip"`
	}

	postUser := user{IP: "invalid IP"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	messages := MapData{
		"ip": []string{"ip:custom_message"},
	}

	rules := MapData{
		"ip": []string{"ip"},
	}

	opts := Options{
		Request:  req,
		Data:     &userObj,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if validationErr.Get("ip") != "custom_message" {
		t.Error("IP custom message failed!")
	}
}

func Test_IPv4(t *testing.T) {
	type user struct {
		IP string `json:"ip"`
	}

	postUser := user{IP: "invalid IP"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"ip": []string{"ip_v4"},
	}

	opts := Options{
		Request: req,
		Data:    &userObj,
		Rules:   rules,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 1 {
		t.Log(validationErr)
		t.Error("IP v4 validation failed!")
	}
}

func Test_IPv4_message(t *testing.T) {
	type user struct {
		IP string `json:"ip"`
	}

	postUser := user{IP: "invalid IP"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	messages := MapData{
		"ip": []string{"ip_v4:custom_message"},
	}

	rules := MapData{
		"ip": []string{"ip_v4"},
	}

	opts := Options{
		Request:  req,
		Data:     &userObj,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if validationErr.Get("ip") != "custom_message" {
		t.Error("IP v4 custom message failed!")
	}
}

func Test_IPv6(t *testing.T) {
	type user struct {
		IP string `json:"ip"`
	}

	postUser := user{IP: "invalid IP"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"ip": []string{"ip_v6"},
	}

	opts := Options{
		Request: req,
		Data:    &userObj,
		Rules:   rules,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 1 {
		t.Log(validationErr)
		t.Error("IP v6 validation failed!")
	}
}

func Test_IPv6_message(t *testing.T) {
	type user struct {
		IP string `json:"ip"`
	}

	postUser := user{IP: "invalid IP"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	messages := MapData{
		"ip": []string{"ip_v6:custom_message"},
	}

	rules := MapData{
		"ip": []string{"ip_v6"},
	}

	opts := Options{
		Request:  req,
		Data:     &userObj,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if validationErr.Get("ip") != "custom_message" {
		t.Error("IP v6 custom message failed!")
	}
}

func Test_JSON(t *testing.T) {
	type user struct {
		Settings string `json:"settings"`
	}

	postUser := user{Settings: "invalid json"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"settings": []string{"json"},
	}

	opts := Options{
		Request: req,
		Data:    &userObj,
		Rules:   rules,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 1 {
		t.Log(validationErr)
		t.Error("JSON validation failed!")
	}
}

func Test_JSON_valid(t *testing.T) {
	type user struct {
		Settings string `json:"settings"`
	}

	postUser := user{Settings: `{"name": "John Doe", "age": 30}`}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"settings": []string{"json"},
	}

	opts := Options{
		Request: req,
		Data:    &userObj,
		Rules:   rules,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 0 {
		t.Log(validationErr)
		t.Error("Validation failed for valid JSON")
	}
}

func Test_JSON_message(t *testing.T) {
	type user struct {
		Settings string `json:"settings"`
	}

	postUser := user{Settings: "invalid json"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	messages := MapData{
		"settings": []string{"json:custom_message"},
	}

	rules := MapData{
		"settings": []string{"json"},
	}

	opts := Options{
		Request:  req,
		Data:     &userObj,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if validationErr.Get("settings") != "custom_message" {
		t.Error("JSON custom message failed!")
	}
}

func Test_LatLon(t *testing.T) {
	type Location struct {
		Latitude  string `json:"lat"`
		Longitude string `json:"lon"`
	}

	postLocation := Location{Latitude: "invalid lat", Longitude: "invalid lon"}
	var loc Location

	body, _ := json.Marshal(postLocation)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"lat": []string{"lat"},
		"lon": []string{"lon"},
	}

	opts := Options{
		Request: req,
		Data:    &loc,
		Rules:   rules,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 2 {
		t.Log(validationErr)
		t.Error("Lat Lon validation failed!")
	}
}

func Test_LatLon_valid(t *testing.T) {
	type Location struct {
		Latitude  string `json:"lat"`
		Longitude string `json:"lon"`
	}

	postLocation := Location{Latitude: "23.810332", Longitude: "90.412518"}
	var loc Location

	body, _ := json.Marshal(postLocation)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"lat": []string{"lat"},
		"lon": []string{"lon"},
	}

	opts := Options{
		Request: req,
		Data:    &loc,
		Rules:   rules,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 0 {
		t.Log(validationErr)
		t.Error("Valid Lat Lon validation failed!")
	}
}

func Test_LatLon_message(t *testing.T) {
	type Location struct {
		Latitude  string `json:"lat"`
		Longitude string `json:"lon"`
	}

	postLocation := Location{Latitude: "invalid lat", Longitude: "invalid lon"}
	var loc Location

	body, _ := json.Marshal(postLocation)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	messages := MapData{
		"lat": []string{"lat:custom_message"},
		"lon": []string{"lon:custom_message"},
	}

	rules := MapData{
		"lat": []string{"lat"},
		"lon": []string{"lon"},
	}

	opts := Options{
		Request:  req,
		Data:     &loc,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if validationErr.Get("lat") != "custom_message" ||
		validationErr.Get("lon") != "custom_message" {
		t.Error("Lat lon custom message failed")
	}
}

func Test_Len(t *testing.T) {
	type user struct {
		Name        string   `json:"name"`
		Roll        int      `json:"roll"`
		Permissions []string `json:"permissions"`
	}

	postUser := user{
		Name:        "john",
		Roll:        11,
		Permissions: []string{"create", "delete", "update"},
	}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"name":        []string{"len:5"},
		"roll":        []string{"len:5"},
		"permissions": []string{"len:10"},
	}

	opts := Options{
		Request: req,
		Data:    &userObj,
		Rules:   rules,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 3 {
		t.Log(validationErr)
		t.Error("Len validation failed!")
	}
}

func Test_Len_message(t *testing.T) {
	type user struct {
		Name        string   `json:"name"`
		Roll        int      `json:"roll"`
		Permissions []string `json:"permissions"`
	}

	postUser := user{
		Name:        "john",
		Roll:        11,
		Permissions: []string{"create", "delete", "update"},
	}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	messages := MapData{
		"name":        []string{"len:custom_message"},
		"roll":        []string{"len:custom_message"},
		"permissions": []string{"len:custom_message"},
	}

	rules := MapData{
		"name":        []string{"len:5"},
		"roll":        []string{"len:5"},
		"permissions": []string{"len:10"},
	}

	opts := Options{
		Request:  req,
		Data:     &userObj,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if validationErr.Get("name") != "custom_message" ||
		validationErr.Get("roll") != "custom_message" ||
		validationErr.Get("permissions") != "custom_message" {
		t.Error("len custom message failed")
	}
}

func Test_Numeric(t *testing.T) {
	type user struct {
		NID string `json:"nid"`
	}

	postUser := user{NID: "invalid nid"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"nid": []string{"numeric"},
	}

	messages := MapData{
		"nid": []string{"numeric:custom_message"},
	}

	opts := Options{
		Request:  req,
		Data:     &userObj,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 1 {
		t.Log(validationErr)
		t.Error("Numeric validation failed!")
	}

	if validationErr.Get("nid") != "custom_message" {
		t.Error("Numeric custom message failed!")
	}
}

func Test_Numeric_valid(t *testing.T) {
	type user struct {
		NID string `json:"nid"`
	}

	postUser := user{NID: "109922"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"nid": []string{"numeric"},
	}

	messages := MapData{
		"nid": []string{"numeric:custom_message"},
	}

	opts := Options{
		Request:  req,
		Data:     &userObj,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 0 {
		t.Log(validationErr)
		t.Error("Valid numeric validation failed!")
	}
}

func Test_NumericBetween(t *testing.T) {
	type user struct {
		Age  int    `json:"age"`
		CGPA string `json:"cgpa"`
	}

	postUser := user{Age: 77, CGPA: "2.90"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"age":  []string{"numeric_between:18,60"},
		"cgpa": []string{"numeric_between:3.5,4.9"},
	}

	messages := MapData{
		"age":  []string{"numeric_between:custom_message"},
		"cgpa": []string{"numeric_between:custom_message"},
	}

	opts := Options{
		Request:  req,
		Data:     &userObj,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 2 {
		t.Error("numeric_between validation failed!")
	}

	if validationErr.Get("age") != "custom_message" ||
		validationErr.Get("cgpa") != "custom_message" {
		t.Error("numeric_between custom message failed!")
	}
}

func Test_URL(t *testing.T) {
	type user struct {
		Web string `json:"web"`
	}

	postUser := user{Web: "invalid url"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"web": []string{"url"},
	}

	messages := MapData{
		"web": []string{"url:custom_message"},
	}

	opts := Options{
		Request:  req,
		Data:     &userObj,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 1 {
		t.Log(validationErr)
		t.Error("URL validation failed!")
	}

	if validationErr.Get("web") != "custom_message" {
		t.Error("URL custom message failed!")
	}
}

func Test_UR_valid(t *testing.T) {
	type user struct {
		Web string `json:"web"`
	}

	postUser := user{Web: "www.google.com"}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"web": []string{"url"},
	}

	messages := MapData{
		"web": []string{"url:custom_message"},
	}

	opts := Options{
		Request:  req,
		Data:     &userObj,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 0 {
		t.Error("Valid URL validation failed!")
	}
}

func Test_UUIDS(t *testing.T) {
	type user struct {
		UUID   string `json:"uuid"`
		UUIDV3 string `json:"uuid3"`
		UUIDV4 string `json:"uuid4"`
		UUIDV5 string `json:"uuid5"`
	}

	postUser := user{
		UUID:   "invalid uuid",
		UUIDV3: "invalid uuid",
		UUIDV4: "invalid uuid",
		UUIDV5: "invalid uuid",
	}
	var userObj user

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"uuid":  []string{"uuid"},
		"uuid3": []string{"uuid_v3"},
		"uuid4": []string{"uuid_v4"},
		"uuid5": []string{"uuid_v5"},
	}

	messages := MapData{
		"uuid":  []string{"uuid:custom_message"},
		"uuid3": []string{"uuid_v3:custom_message"},
		"uuid4": []string{"uuid_v4:custom_message"},
		"uuid5": []string{"uuid_v5:custom_message"},
	}

	opts := Options{
		Request:  req,
		Data:     &userObj,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 4 {
		t.Error("UUID validation failed!")
	}

	if validationErr.Get("uuid") != "custom_message" ||
		validationErr.Get("uuid3") != "custom_message" ||
		validationErr.Get("uuid4") != "custom_message" ||
		validationErr.Get("uuid5") != "custom_message" {
		t.Error("UUID custom message failed!")
	}

}

func Test_min(t *testing.T) {
	type Body struct {
		Str     string   `json:"_str"`
		Slice   []string `json:"_slice"`
		Int     int      `json:"_int"`
		Int8    int8     `json:"_int8"`
		Int16   int16    `json:"_int16"`
		Int32   int32    `json:"_int32"`
		Int64   int64    `json:"_int64"`
		Uint    uint     `json:"_uint"`
		Uint8   uint8    `json:"_uint8"`
		Uint16  uint16   `json:"_uint16"`
		Uint32  uint32   `json:"_uint32"`
		Uint64  uint64   `json:"_uint64"`
		Uintptr uintptr  `json:"_uintptr"`
		Float32 float32  `json:"_float32"`
		Float64 float64  `json:"_float64"`
	}

	postBody := Body{
		Str:     "xyz",
		Slice:   []string{"x", "y"},
		Int:     2,
		Int8:    2,
		Int16:   2,
		Int32:   2,
		Int64:   2,
		Uint:    2,
		Uint8:   2,
		Uint16:  2,
		Uint32:  2,
		Uint64:  2,
		Uintptr: 2,
		Float32: 2.4,
		Float64: 3.2,
	}

	var bodyObj Body

	body, _ := json.Marshal(postBody)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"_str":     []string{"min:5"},
		"_slice":   []string{"min:5"},
		"_int":     []string{"min:5"},
		"_int8":    []string{"min:5"},
		"_int16":   []string{"min:5"},
		"_int32":   []string{"min:5"},
		"_int64":   []string{"min:5"},
		"_uint":    []string{"min:5"},
		"_uint8":   []string{"min:5"},
		"_uint16":  []string{"min:5"},
		"_uint32":  []string{"min:5"},
		"_uint64":  []string{"min:5"},
		"_uintptr": []string{"min:5"},
		"_float32": []string{"min:5"},
		"_float64": []string{"min:5"},
	}

	messages := MapData{
		"_str":     []string{"min:custom_message"},
		"_slice":   []string{"min:custom_message"},
		"_int":     []string{"min:custom_message"},
		"_uint":    []string{"min:custom_message"},
		"_float32": []string{"min:custom_message"},
	}

	opts := Options{
		Request:  req,
		Data:     &bodyObj,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 15 {
		t.Error("min validation failed!")
	}

	if validationErr.Get("_str") != "custom_message" ||
		validationErr.Get("_slice") != "custom_message" ||
		validationErr.Get("_int") != "custom_message" ||
		validationErr.Get("_uint") != "custom_message" ||
		validationErr.Get("_float32") != "custom_message" {
		t.Error("min custom message failed!")
	}
}

func Test_max(t *testing.T) {
	type Body struct {
		Str     string   `json:"_str"`
		Slice   []string `json:"_slice"`
		Int     int      `json:"_int"`
		Int8    int8     `json:"_int8"`
		Int16   int16    `json:"_int16"`
		Int32   int32    `json:"_int32"`
		Int64   int64    `json:"_int64"`
		Uint    uint     `json:"_uint"`
		Uint8   uint8    `json:"_uint8"`
		Uint16  uint16   `json:"_uint16"`
		Uint32  uint32   `json:"_uint32"`
		Uint64  uint64   `json:"_uint64"`
		Uintptr uintptr  `json:"_uintptr"`
		Float32 float32  `json:"_float32"`
		Float64 float64  `json:"_float64"`
	}

	postBody := Body{
		Str:     "xyzabc",
		Slice:   []string{"x", "y", "z"},
		Int:     20,
		Int8:    20,
		Int16:   20,
		Int32:   20,
		Int64:   20,
		Uint:    20,
		Uint8:   20,
		Uint16:  20,
		Uint32:  20,
		Uint64:  20,
		Uintptr: 20,
		Float32: 20.4,
		Float64: 30.2,
	}

	var bodyObj Body

	body, _ := json.Marshal(postBody)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	rules := MapData{
		"_str":     []string{"max:5"},
		"_slice":   []string{"max:2"},
		"_int":     []string{"max:5"},
		"_int8":    []string{"max:5"},
		"_int16":   []string{"max:5"},
		"_int32":   []string{"max:5"},
		"_int64":   []string{"max:5"},
		"_uint":    []string{"max:5"},
		"_uint8":   []string{"max:5"},
		"_uint16":  []string{"max:5"},
		"_uint32":  []string{"max:5"},
		"_uint64":  []string{"max:5"},
		"_uintptr": []string{"max:5"},
		"_float32": []string{"max:5"},
		"_float64": []string{"max:5"},
	}

	messages := MapData{
		"_str":     []string{"max:custom_message"},
		"_slice":   []string{"max:custom_message"},
		"_int":     []string{"max:custom_message"},
		"_uint":    []string{"max:custom_message"},
		"_float32": []string{"max:custom_message"},
	}

	opts := Options{
		Request:  req,
		Data:     &bodyObj,
		Rules:    rules,
		Messages: messages,
	}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 15 {
		t.Error(validationErr)
		t.Error("max validation failed!")
	}

	if validationErr.Get("_str") != "custom_message" ||
		validationErr.Get("_slice") != "custom_message" ||
		validationErr.Get("_int") != "custom_message" ||
		validationErr.Get("_uint") != "custom_message" ||
		validationErr.Get("_float32") != "custom_message" {
		t.Error("max custom message failed!")
	}
}
