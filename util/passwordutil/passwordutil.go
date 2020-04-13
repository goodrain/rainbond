package passwordutil

import (
	"crypto/sha256"
	"fmt"
)

//EncryptionPassword encryption password
func EncryptionPassword(password, email string) (string, error) {
	password = email + password
	new := fmt.Sprintf("%d", int(password[7])) + password + fmt.Sprintf("%d", int(password[5])) + "goodrain" + fmt.Sprintf("%d", int(password[2])/7)
	h := sha256.New224()
	if _, err := h.Write([]byte(new)); err != nil {
		return "", err
	}
	res := fmt.Sprintf("%x", h.Sum(nil))
	return res[0:16], nil
}

//CheckPassword check password
func CheckPassword(password, storepass, email string) bool {
	ep, err := EncryptionPassword(password, email)
	if err != nil {
		return false
	}
	return ep == storepass
}
