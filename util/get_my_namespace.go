package util

import "io/ioutil"

const namespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

func GetMyNamespace() (string, error) {
	data, err := ioutil.ReadFile(namespaceFile)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
