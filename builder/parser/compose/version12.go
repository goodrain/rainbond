// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

package compose

import "fmt"

// parseV1V2 now reuses the compose-go based parser so we do not rely on the
// legacy libcompose dependency chain during Kubernetes and KubeVirt upgrades.
func parseV1V2(bodys [][]byte) (ComposeObject, error) {
	co, _, err := parseSpec(bodys, "")
	if err != nil {
		return ComposeObject{}, fmt.Errorf("parse compose v1/v2 with compose-go failed: %s", err.Error())
	}
	return co, nil
}
