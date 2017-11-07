package govalidator

import "testing"

type inputs map[string]bool

var (
	_alpha = inputs{
		"abcdefghijgklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ": true,
		"7877**": false,
		"abc":    true,
		")(^%&)": false,
	}
	_alphaDash = inputs{
		"abcdefghijgklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_-": true,
		"John_Do-E": true,
		"+=a(0)":    false,
	}
	_alphaNumeric = inputs{
		"abcdefghijgklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ01234567890": true,
		"090a": true,
		"*&*)": false,
	}
	_boolStringsList = inputs{
		"0":     true,
		"1":     true,
		"true":  true,
		"false": true,
		"o":     false,
		"a":     false,
	}
	// Ref: https://www.freeformatter.com/credit-card-number-generator-validator.html
	_creditCardList = inputs{
		"4896644531043572": true,
		"2221005631780408": true,
		"349902515380498":  true,
		"6011843157272458": true,
		"3543358904915048": true,
		"5404269782892303": true,
		"4508168417293390": true,
		"0604595245598387": false,
		"6388244169973297": false,
	}
	_coordinateList = inputs{
		"30.297018,-78.486328": true,
		"40.044438,-104.0625":  true,
		"58.068581,-99.580078": true,
		"abc, xyz":             false,
		"0, 887":               false,
	}
	_cssColorList = inputs{
		"#000":           true,
		"#00aaff":        true,
		"rgb(123,32,12)": true,
		"#0":             false,
		"#av":            false,
	}
	_dateList = inputs{
		"2016-10-14": true,
		"2013/02/18": true,
		"2020/12/30": true,
		"0001/14/30": false,
	}
	_dateDDMMYYList = inputs{
		"01-01-2000": true,
		"28/02/2001": true,
		"01/12/2000": true,
		"2012/11/30": false,
		"201/11/30":  false,
	}
	_emailList = inputs{
		"john@example.com":       true,
		"thedevsaddam@gmail.com": true,
		"jane@yahoo.com":         true,
		"janeahoo.com":           false,
		"janea@.com":             false,
	}
	_floatList         = inputs{"123": true, "12.50": true, "33.07": true, "abc": false, "o0.45": false}
	_roleList          = []string{"admin", "manager", "supervisor"}
	_validJSONString   = `{"FirstName": "Bob", "LastName": "Smith"}`
	_invalidJSONString = `{"invalid json"}`
	_numericStringList = inputs{"12": true, "09": true, "878": true, "100": true, "a": false, "xyz": false}
	_latList           = inputs{"30.297018": true, "40.044438": true, "a": false, "xyz": false}
	_lonList           = inputs{"-78.486328": true, "-104.0625": true, "a": false, "xyz": false}
	_ipList            = inputs{"10.255.255.255": true, "172.31.255.255": true, "192.168.255.255": true, "a92.168.255.255": false, "172.31.255.25b": false}
	_ipV6List          = inputs{
		"1200:0000:AB00:1234:0000:2552:7777:1313": true,
		"21DA:D3:0:2F3B:2AA:FF:FE28:9C5A":         true,
		"10.255.255.255":                          false,
	}
	_urlList = inputs{
		"http://www.google.com":  true,
		"https://www.google.com": true,
		"https://facebook.com":   true,
		"yahoo.com":              true,
		"adca":                   false,
	}
	_uuidList = inputs{
		"ee7cf0a0-1922-401b-a1ae-6ec9261484c0": true,
		"ee7cf0a0-1922-401b-a1ae-6ec9261484c1": true,
		"ee7cf0a0-1922-401b-a1ae-6ec9261484a0": true,
		"39888f87-fb62-5988-a425-b2ea63f5b81e": false,
	}
	_uuidV3List = inputs{
		"a987fbc9-4bed-3078-cf07-9141ba07c9f3": true,
		"b987fbc9-4bed-3078-cf07-9141ba07c9f3": true,
		"ee7cf0a0-1922-401b-a1ae-6ec9261484c0": false,
	}
	_uuidV4List = inputs{
		"df7cca36-3d7a-40f4-8f06-ae03cc22f045": true,
		"ef7cca36-3d7a-40f4-8f06-ae03cc22f048": true,
		"b987fbc9-4bed-3078-cf07-9141ba07c9f3": false,
	}
	_uuidV5List = inputs{
		"39888f87-fb62-5988-a425-b2ea63f5b81e": true,
		"33388f87-fb62-5988-a425-b2ea63f5b81f": true,
		"b987fbc9-4bed-3078-cf07-9141ba07c9f3": false,
	}
)

func Test_IsAlpha(t *testing.T) {
	for a, s := range _alpha {
		if isAlpha(a) != s {
			t.Error("IsAlpha failed to determine alpha!")
		}
	}
}

func Benchmark_IsAlpha(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isAlpha("abcdAXZY")
	}
}

func Test_IsAlphaDash(t *testing.T) {
	for a, s := range _alphaDash {
		if isAlphaDash(a) != s {
			t.Error("IsAlphaDash failed to determine alpha dash!")
		}
	}
}

func Benchmark_IsAlphaDash(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isAlphaDash("John_Do-E")
	}
}

func Test_IsAlphaNumeric(t *testing.T) {
	for a, s := range _alphaNumeric {
		if isAlphaNumeric(a) != s {
			t.Error("IsAlphaNumeric failed to determine alpha numeric!")
		}
	}
}

func Benchmark_IsAlphaNumeric(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isAlphaNumeric("abc12AZ")
	}
}

func Test_IsBoolean(t *testing.T) {
	for b, s := range _boolStringsList {
		if isBoolean(b) != s {
			t.Error("IsBoolean failed to determine boolean!")
		}
	}
}

func Benchmark_IsBoolean(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isBoolean("true")
	}
}

func Test_IsCreditCard(t *testing.T) {
	for card, state := range _creditCardList {
		if isCreditCard(card) != state {
			t.Error("IsCreditCard failed to determine credit card!")
		}
	}
}

func Benchmark_IsCreditCard(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isCreditCard("2221005631780408")
	}
}

func Test_IsCoordinate(t *testing.T) {
	for c, s := range _coordinateList {
		if isCoordinate(c) != s {
			t.Error("IsCoordinate failed to determine coordinate!")
		}
	}
}

func Benchmark_IsCoordinate(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isCoordinate("30.297018,-78.486328")
	}
}

func Test_IsCSSColor(t *testing.T) {
	for c, s := range _cssColorList {
		if isCSSColor(c) != s {
			t.Error("IsCSSColor failed to determine css color code!")
		}
	}
}

func Benchmark_IsCSSColor(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isCSSColor("#00aaff")
	}
}

func Test_IsDate(t *testing.T) {
	for d, s := range _dateList {
		if isDate(d) != s {
			t.Error("IsDate failed to determine date!")
		}
	}
}

func Benchmark_IsDate(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isDate("2016-10-14")
	}
}

func Test_IsDateDDMMYY(t *testing.T) {
	for d, s := range _dateDDMMYYList {
		if isDateDDMMYY(d) != s {
			t.Error("IsDateDDMMYY failed to determine date!")
		}
	}
}

func Benchmark_IsDateDDMMYY(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isDateDDMMYY("23-10-2014")
	}
}

func Test_IsEmail(t *testing.T) {
	for e, s := range _emailList {
		if isEmail(e) != s {
			t.Error("IsEmail failed to determine email!")
		}
	}
}

func Benchmark_IsEmail(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isEmail("thedevsaddam@gmail.com")
	}
}

func Test_IsFloat(t *testing.T) {
	for f, s := range _floatList {
		if isFloat(f) != s {
			t.Error("IsFloat failed to determine float value!")
		}
	}
}

func Benchmark_IsFloat(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isFloat("123.001")
	}
}

func Test_IsIn(t *testing.T) {
	if !isIn(_roleList, "admin") {
		t.Error("IsIn failed!")
	}
}

func Benchmark_IsIn(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isIn(_roleList, "maager")
	}
}

func Test_IsJSON(t *testing.T) {
	if !isJSON(_validJSONString) {
		t.Error("IsJSON failed!")
	}
	if isJSON(_invalidJSONString) {
		t.Error("IsJSON unable to detect invalid json!")
	}
}

func Benchmark_IsJSON(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isJSON(_validJSONString)
	}
}

func Test_IsNumeric(t *testing.T) {
	for n, s := range _numericStringList {
		if isNumeric(n) != s {
			t.Error("IsNumeric failed!")
		}
	}
}

func Benchmark_IsNumeric(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isNumeric("123")
	}
}

func Test_IsLatitude(t *testing.T) {
	for n, s := range _latList {
		if isLatitude(n) != s {
			t.Error("IsLatitude failed!")
		}
	}
}

func Benchmark_IsLatitude(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isLatitude("30.297018")
	}
}

func Test_IsLongitude(t *testing.T) {
	for n, s := range _lonList {
		if isLongitude(n) != s {
			t.Error("IsLongitude failed!")
		}
	}
}

func Benchmark_IsLongitude(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isLongitude("-78.486328")
	}
}

func Test_IsIP(t *testing.T) {
	for i, s := range _ipList {
		if isIP(i) != s {
			t.Error("IsIP failed!")
		}
	}
}

func Benchmark_IsIP(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isIP("10.255.255.255")
	}
}

func Test_IsIPV4(t *testing.T) {
	for i, s := range _ipList {
		if isIPV4(i) != s {
			t.Error("IsIPV4 failed!")
		}
	}
}

func Benchmark_IsIPV4(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isIPV4("10.255.255.255")
	}
}

func Test_IsIPV6(t *testing.T) {
	for i, s := range _ipV6List {
		if isIPV6(i) != s {
			t.Error("IsIPV4 failed!")
		}
	}
}

func Benchmark_IsIPV6(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isIPV6("10.255.255.255")
	}
}

func Test_IsMatchedRegex(t *testing.T) {
	if !isMatchedRegex("^(name|age)$", "name") {
		t.Error("IsMatchedRegex failed!")
	}
}

func Benchmark_IsMatchedRegex(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isMatchedRegex("^(name|age)$", "name")
	}
}

func Test_IsURL(t *testing.T) {
	for u, s := range _urlList {
		if isURL(u) != s {
			t.Error("IsURL failed!")
		}
	}
}

func Benchmark_IsURL(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isURL("https://www.facebook.com")
	}
}

func Test_IsUUID(t *testing.T) {
	for u, s := range _uuidList {
		if isUUID(u) != s {
			t.Error("IsUUID failed!")
		}
	}
}

func Benchmark_IsUUID(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isUUID("ee7cf0a0-1922-401b-a1ae-6ec9261484c0")
	}
}

func Test_IsUUID3(t *testing.T) {
	for u, s := range _uuidV3List {
		if isUUID3(u) != s {
			t.Error("IsUUID3 failed!")
		}
	}
}

func Benchmark_IsUUID3(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isUUID3("a987fbc9-4bed-3078-cf07-9141ba07c9f3")
	}
}

func Test_IsUUID4(t *testing.T) {
	for u, s := range _uuidV4List {
		if isUUID4(u) != s {
			t.Error("IsUUID4 failed!")
		}
	}
}

func Benchmark_IsUUID4(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isUUID4("57b73598-8764-4ad0-a76a-679bb6640eb1")
	}
}

func Test_IsUUID5(t *testing.T) {
	for u, s := range _uuidV5List {
		if isUUID5(u) != s {
			t.Error("IsUUID5 failed!")
		}
	}
}

func Benchmark_IsUUID5(b *testing.B) {
	for n := 0; n < b.N; n++ {
		isUUID5("987fbc97-4bed-5078-9f07-9141ba07c9f3")
	}
}
