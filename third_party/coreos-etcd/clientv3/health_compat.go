package clientv3

import (
	"context"
	"fmt"
	"net"
)

func getHost(endpoint string) string {
	_, host, _ := parseEndpoint(endpoint)
	return host
}

func grpcHealthCheck(c *Client, ep string) (bool, error) {
	proto, host, _ := parseEndpoint(ep)
	if proto == "" || host == "" {
		return false, fmt.Errorf("invalid endpoint %q", ep)
	}

	timeout := c.cfg.DialTimeout
	if timeout <= 0 {
		timeout = minHealthRetryDuration
	}

	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()

	conn, err := (&net.Dialer{}).DialContext(ctx, proto, host)
	if err != nil {
		return false, err
	}
	_ = conn.Close()
	return true, nil
}
