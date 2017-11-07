package govalidator

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
)

func TestValidator_SetDefaultRequired(t *testing.T) {
	v := New(Options{})
	v.SetDefaultRequired(true)
	if !v.Opts.RequiredDefault {
		t.Error("SetDefaultRequired failed")
	}
}

func TestValidator_Validate(t *testing.T) {
	var URL *url.URL
	URL, _ = url.Parse("http://www.example.com")
	params := url.Values{}
	params.Add("name", "John Doe")
	params.Add("username", "jhondoe")
	params.Add("email", "john@mail.com")
	params.Add("zip", "8233")
	URL.RawQuery = params.Encode()
	r, _ := http.NewRequest("GET", URL.String(), nil)
	rulesList := MapData{
		"name":  []string{"required"},
		"age":   []string{"between:5,16"},
		"email": []string{"email"},
		"zip":   []string{"digits:4"},
	}

	opts := Options{
		Request: r,
		Rules:   rulesList,
	}
	v := New(opts)
	validationError := v.Validate()
	if len(validationError) > 0 {
		t.Error("Validate failed to validate correct inputs!")
	}

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Validate did not panic")
		}
	}()

	v1 := New(Options{Rules: MapData{}})
	v1.Validate()
}

func Benchmark_Validate(b *testing.B) {
	var URL *url.URL
	URL, _ = url.Parse("http://www.example.com")
	params := url.Values{}
	params.Add("name", "John Doe")
	params.Add("age", "27")
	params.Add("email", "john@mail.com")
	params.Add("zip", "8233")
	URL.RawQuery = params.Encode()
	r, _ := http.NewRequest("GET", URL.String(), nil)
	rulesList := MapData{
		"name":  []string{"required"},
		"age":   []string{"numeric_between:18,60"},
		"email": []string{"email"},
		"zip":   []string{"digits:4"},
	}

	opts := Options{
		Request: r,
		Rules:   rulesList,
	}
	v := New(opts)
	for n := 0; n < b.N; n++ {
		v.Validate()
	}
}

//============ validate json test ====================

func TestValidator_ValidateJSON(t *testing.T) {
	type User struct {
		Name    string `json:"name"`
		Email   string `json:"email"`
		Address string `json:"address"`
		Age     int    `json:"age"`
		Zip     string `json:"zip"`
		Color   int    `json:"color"`
	}

	postUser := User{
		Name:    "",
		Email:   "inalid email",
		Address: "",
		Age:     1,
		Zip:     "122",
		Color:   5,
	}

	rules := MapData{
		"name":    []string{"required"},
		"email":   []string{"email"},
		"address": []string{"required", "between:3,5"},
		"age":     []string{"bool"},
		"zip":     []string{"len:4"},
		"color":   []string{"min:10"},
	}

	var user User

	body, _ := json.Marshal(postUser)
	req, _ := http.NewRequest("POST", "http://www.example.com", bytes.NewReader(body))

	opts := Options{
		Request: req,
		Data:    &user,
		Rules:   rules,
	}

	vd := New(opts)
	vd.SetTagIdentifier("json")
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 5 {
		t.Error("ValidateJSON failed")
	}
}

func TestValidator_ValidateJSON_panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("ValidateJSON did not panic")
		}
	}()

	opts := Options{}

	vd := New(opts)
	validationErr := vd.ValidateJSON()
	if len(validationErr) != 5 {
		t.Error("ValidateJSON failed")
	}
}
