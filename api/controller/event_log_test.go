package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capability_id: rainbond.event-log.require-log-identity
func TestLogListRejectsMissingLogIdentity(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v2/cluster/log-file", nil)

	require.NotPanics(t, func() {
		(&EventLogStruct{}).LogList(recorder, request)
	})
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}
