### 用法实例
```
type User struct {
	Username string `validate:"username|required|between:3,8"`
	Email    string `validate:"email|email"`
	Web      string `validate:"web|url"`
	Age      int    `validate:"age|numeric_between:18,55"`
}

func handler(w http.ResponseWriter, r *http.Request) {
	var user User
	messages := validator.MapData{
		"username": []string{"required:You must provide username", "between:username must be between 3 to 8 chars"},
		"web":      []string{"url:You must provide a valid url"},
	}

	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &user, messages) {
		return
	}
	httputil.ReturnSuccess(r, w, user)
}
func handlerMap(w http.ResponseWriter, r *http.Request) {
	rule := validator.MapData{
		"username": []string{"required", "between:3,5"},
		"web":      []string{"url"},
	}
	messages := validator.MapData{
		"username": []string{"required:You must provide username", "between:username must be between 3 to 8 chars"},
		"web":      []string{"url:You must provide a valid url"},
	}
	data, ok := httputil.ValidatorRequestMapAndErrorResponse(r, w, rule, messages)
	if !ok {
		return
	}
	httputil.ReturnSuccess(r, w, data)
}
```
### Validation Rules
* `alpha` The field under validation must be entirely alphabetic characters.
* `alpha_dash` The field under validation may have alpha-numeric characters, as well as dashes and underscores.
* `alpha_num` The field under validation must be entirely alpha-numeric characters.
* `numeric` The field under validation must be entirely numeric characters.
* `numeric_between:int,int` The field under validation must be a numeric value between the range.
   e.g: `numeric_between:18,65` may contains numeric value like `35`, `55`
* `bool` The field under validation must be able to be cast as a boolean. Accepted input are `true, false, 1, 0, "1" and "0"`.
* `credit_card` The field under validation must have a valid credit card number. Accepted cards are `Visa, MasterCard, American Express, Diners Club, Discover and JCB card`
* `coordinate` The field under validation must have a value of valid coordinate.
* `css_color` The field under validation must have a value of valid CSS color. Accepted colors are `hex, rgb, rgba, hsl, hsla` like `#909, #00aaff, rgb(255,122,122)`
* `date` The field under validation must have a valid date of format yyyy-mm-dd or yyyy/mm/dd.
* `date:dd-mm-yyyy` The field under validation must have a valid date of format dd-mm-yyyy.
* `digits:int` The field under validation must be numeric and must have an exact length of value.
* `digits_between:int,int` The field under validation must be numeric and must have length between the range.
   e.g: `digits_between:3,5` may contains digits like `2323`, `12435`
* `email` The field under validation must have a valid email.
* `float` The field under validation must have a valid float number.
* `in:foo,bar` The field under validation must have one of the values. e.g: `in:admin,manager,user` must contain the values (admin or manager or user)
* `min:int` The field under validation must have a min length of characters.
   e.g: `min:3` may contains characters minimum length of 3 like `"john", "jane", "jane321"` but not `"mr", "xy"`
* `max:int` The field under validation must have a max length of characters.
   e.g: `max:6` may contains characters maximum length of 6 like `"john doe", "jane doe"` but not `"john", "jane"`
* `not_in:foo,bar` The field under validation must have one value except foo,bar. e.g: `not_in:admin,manager,user` must not contain the values (admin or manager or user)
* `len:int` The field under validation must have an exact length of characters.
   e.g: `len:4` may contains characters exact length of 4 like `Food, Mood, Good`
* `ip` The field under validation must be a valid IP address.
* `ip_v4` The field under validation must be a valid IP V4 address.
* `ip_v6` The field under validation must be a valid IP V6 address.
* `json` The field under validation must be a valid JSON string.
* `lat` The field under validation must be a valid latitude.
* `lon` The field under validation must be a valid longitude.
* `regex:regurlar expression` The field under validation validate against the regex. e.g: `regex:^[a-zA-Z]+$` validate the letters.
* `required` The field under validation must be present in the input data and not empty. A field is considered "empty" if one of the following conditions are true: 1) The value is null. 2)The value is an empty string.
* `url` The field under validation must be a valid URL.
* `uuid` The field under validation must be a valid UUID.
* `uuid_v3` The field under validation must be a valid UUID V3.
* `uuid_v4` The field under validation must be a valid UUID V4.
* `uuid_v5` The field under validation must be a valid UUID V5.
