package prober

import "testing"

func TestParseTCPHostAddress(t *testing.T) {
	re := parseTCPHostAddress("rm-2ze0xlsi14xz6q6sz.mysql.rds.aliyuncs.com", 3306)
	t.Log(re)
}
