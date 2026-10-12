//go:build linux || darwin

package main

import "crypto/subtle"

func registryPermitKeyReader(controlPath, permitPath string, signed bool) (func() []byte, error) {
	if signed && permitPath == "" {
		return nil, errConfiguration
	}
	if permitPath == "" {
		permitPath = controlPath
	}
	read := func() []byte {
		permit, err := readCredential(permitPath)
		if err != nil {
			return nil
		}
		if signed {
			control, err := readCredential(controlPath)
			if err != nil || subtle.ConstantTimeCompare([]byte(control), []byte(permit)) == 1 {
				return nil
			}
		}
		return []byte(permit)
	}
	if len(read()) == 0 {
		return nil, errConfiguration
	}
	return read, nil
}
