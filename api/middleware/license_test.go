package middleware

import "testing"

func TestVerifyLicense(t *testing.T) {
	licPath := "testdata/license.yb"
	licSoPath := "wrong path"
	if err := verifyLicense(licPath, licSoPath); err == nil {
		t.Errorf("expected error, but return nil")
	}
}