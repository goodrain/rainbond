package handler

import (
	"github.com/goodrain/rainbond/api/client/prometheus"
	"testing"
)

func TestGetDiskUsage(t *testing.T) {
	prometheusCli, err := prometheus.NewPrometheus(&prometheus.Options{
		Endpoint: "9999.gr5d40c8.2c9v614j.a24839.grapps.cn",
	})
	if err != nil {
		t.Fatal(err)
	}

	a := ApplicationAction{
		promClient: prometheusCli,
	}

	a.getDiskUsage("4ad713694c934829950f85a26f7c7544")
}
