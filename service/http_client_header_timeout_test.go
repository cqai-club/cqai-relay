package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelayHTTPClientTimesOutWaitingForUpstreamHeaders(t *testing.T) {
	withRelayHTTPTransportSettings(t)
	previousHeaderTimeout, previousTimeout := common.RelayResponseHeaderTimeout, common.RelayTimeout
	common.RelayResponseHeaderTimeout = 1
	common.RelayTimeout = 0
	t.Cleanup(func() {
		common.RelayResponseHeaderTimeout = previousHeaderTimeout
		common.RelayTimeout = previousTimeout
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)
	transport := newRelayHTTPTransport()
	t.Cleanup(transport.CloseIdleConnections)
	client := newRelayHTTPClient(transport)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	require.Error(t, err)
	assert.True(t, os.IsTimeout(err), "the upstream header wait must be bounded")
	assert.NoError(t, ctx.Err(), "the transport must time out before the caller deadline")
}
