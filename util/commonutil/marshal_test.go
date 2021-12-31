package commonutil

import "testing"

func TestMapToString(t *testing.T) {
	m := make(map[string]interface{})
	m["111"] = "111x"
	m["222"] = "222x"

	str, _ := MapToString(m)
	t.Log(str)

	m2 := make(map[string]interface{})
	m2["111"] = User{Name: "dp1", Age: 10}
	m2["222"] = User{Name: "dp12", Age: 20}

	str2, _ := MapToString(m2)
	t.Log(str2)
}

func TestStringToMap(t *testing.T) {
	str := `{"111":"111x","222":"222x"}`
	m, _ := StringToMap(str)
	t.Log(m)

	str2 := `{"111":{"Name":"dp1","Age":10},"222":{"Name":"dp12","Age":20}}`
	m2, _ := StringToMap(str2)
	t.Log(m2)
}

func TestSliceToString(t *testing.T) {
	sli := make([]interface{}, 0)
	sli = append(sli, "111x")
	sli = append(sli, "222x")

	str, _ := SliceToString(sli)
	t.Log(str)

	sli2 := make([]interface{}, 0)
	sli2 = append(sli2, User{Name: "dp1", Age: 10})
	sli2 = append(sli2, User{Name: "dp2", Age: 20})

	str2, _ := SliceToString(sli2)
	t.Log(str2)
}

func TestStringToSlice(t *testing.T) {
	str := `["111x","222x"]`
	sli, _ := StringToSlice(str)
	t.Log(sli)

	str2 := `[{"Name":"dp1","Age":10},{"Name":"dp2","Age":20}]`
	sli2, _ := StringToSlice(str2)
	t.Log(sli2)
}

type User struct {
	Name string
	Age  int
}
