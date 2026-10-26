package controller

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type overviewNodesHandler struct {
	handler.NodesHandler
	ctx          context.Context
	includeStats bool
}

func (h *overviewNodesHandler) ListNodesWithOptions(ctx context.Context, includeStats bool) ([]model.NodeInfo, error) {
	h.ctx = ctx
	h.includeStats = includeStats
	return []model.NodeInfo{}, nil
}

// capability_id: rainbond.overview.nodes-request-context
func TestOverviewNodesRequestPreservesContextAndDefaults(t *testing.T) {
	for _, tc := range []struct {
		query string
		stats bool
	}{{"", true}, {"?include_stats=true", true}, {"?include_stats=false", false}, {"?include_stats=invalid", true}} {
		t.Run(tc.query, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			r := httptest.NewRequest("GET", "/v2/nodes"+tc.query, nil).WithContext(ctx)
			h := &overviewNodesHandler{}
			_, err := listNodesForRequest(r, h)
			require.NoError(t, err)
			assert.Same(t, ctx, h.ctx)
			assert.Equal(t, tc.stats, h.includeStats)
			assert.ErrorIs(t, h.ctx.Err(), context.Canceled)
		})
	}
}
