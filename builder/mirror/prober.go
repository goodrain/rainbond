// RAINBOND, Application Management Platform
// Copyright (C) 2014-2026 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package mirror

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// probeResult records one alive mirror and how fast its /v2/ endpoint answered.
type probeResult struct {
	url     string
	latency time.Duration
}

// Probe checks every candidate's /v2/ registry endpoint concurrently and
// returns only the alive ones, sorted by ascending latency. A mirror is alive
// when /v2/ answers 200 or 401 (an auth challenge still proves a working
// registry frontend). Candidates keep their scheme; entries without one are
// probed via https.
func Probe(ctx context.Context, candidates []string, timeout time.Duration) []string {
	if len(candidates) == 0 {
		return nil
	}
	client := &http.Client{Timeout: timeout}
	results := make([]probeResult, 0, len(candidates))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, candidate := range candidates {
		candidate := candidate
		wg.Add(1)
		go func() {
			defer wg.Done()
			latency, alive := probeOne(ctx, client, candidate)
			if !alive {
				return
			}
			mu.Lock()
			results = append(results, probeResult{url: candidate, latency: latency})
			mu.Unlock()
		}()
	}
	wg.Wait()
	sort.Slice(results, func(i, j int) bool { return results[i].latency < results[j].latency })
	alive := make([]string, 0, len(results))
	for _, r := range results {
		alive = append(alive, r.url)
	}
	return alive
}

func probeOne(ctx context.Context, client *http.Client, mirrorURL string) (time.Duration, bool) {
	endpoint := mirrorURL
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "https://" + endpoint
	}
	endpoint = strings.TrimSuffix(endpoint, "/") + "/v2/"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		logrus.Debugf("probe mirror %s: build request failure: %v", mirrorURL, err)
		return 0, false
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		logrus.Debugf("probe mirror %s failure: %v", mirrorURL, err)
		return 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		logrus.Debugf("probe mirror %s: unexpected status %s", mirrorURL, resp.Status)
		return 0, false
	}
	return time.Since(start), true
}
