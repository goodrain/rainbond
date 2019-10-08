package license

import "testing"

func TestVerifyTime(t *testing.T) {
	tests := []struct {
		name, licPath, licSoPath string
		exp                      bool
	}{
		{name: "dummy license", licPath: "dummy license", licSoPath: "/opt/rainbond/etc/license/license.so", exp: false},
		{name: "wrong license", licPath: "testdata/wrong_license.yb", licSoPath: "/opt/rainbond/etc/license/license.so", exp: false},
		{name: "wrong license.so", licPath: "testdata/ok_license.yb", licSoPath: "dummy license.so", exp: false},
		{name: "ok", licPath: "testdata/ok_license.yb", licSoPath: "/opt/rainbond/etc/license/license.so", exp: true},
		{name: "expire", licPath: "testdata/expire_license.yb", licSoPath: "/opt/rainbond/etc/license/license.so", exp: false},
	}
	for idx := range tests {
		tc := tests[idx]
		t.Run(tc.name, func(t *testing.T) {
			if res := VerifyTime(tc.licPath, tc.licSoPath); res != tc.exp {
				t.Errorf("Expected %v, but return %v", tc.exp, res)
			}
		})
	}
}

func TestVerifyNodes(t *testing.T) {
	tests := []struct {
		name, licPath, licSoPath string
		curNodes                 int
		exp                      bool
	}{
		{name: "dummy license", licPath: "dummy license", licSoPath: "/opt/rainbond/etc/license/license.so", exp: false},
		{name: "wrong license", licPath: "testdata/wrong_license.yb", licSoPath: "/opt/rainbond/etc/license/license.so", exp: false},
		{name: "wrong license.so", licPath: "testdata/ok_license.yb", licSoPath: "dummy license.so", exp: false},
		{name: "ok", licPath: "testdata/ok_license.yb", licSoPath: "/opt/rainbond/etc/license/license.so", curNodes: 998, exp: true},
		{name: "expire", licPath: "testdata/expire_license.yb", licSoPath: "/opt/rainbond/etc/license/license.so", exp: false},
		{name: "wrong node numbers", licPath: "testdata/ok_license.yb", licSoPath: "/opt/rainbond/etc/license/license.so", curNodes: 999, exp: false},
	}
	for idx := range tests {
		tc := tests[idx]
		t.Run(tc.name, func(t *testing.T) {
			if res := VerifyNodes(tc.licPath, tc.licSoPath, tc.curNodes); res != tc.exp {
				t.Errorf("Expected %v, but return %v", tc.exp, res)
			}
		})
	}
}

func TestGetLicInfo(t *testing.T) {
	licPath := "testdata/ok_license.yb"
	licSoPath := "/opt/rainbond/etc/license/license.so"
	licInfo, err := GetLicInfo(licPath, licSoPath)
	if err != nil {
		t.Error(err)
	}
	t.Logf("%+v", licInfo)
}
