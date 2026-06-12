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

// Package mirror maintains a dynamically refreshed list of docker.io registry
// mirrors. Candidates come from a remote JSON source, are filtered by a live
// /v2/ probe and exposed to the build paths via Manager.
package mirror

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// sourceSchemaVersion is the only mirrors.json schema version this builder
// understands. Any other version is treated as a fetch failure so the last
// known good list keeps being used.
const sourceSchemaVersion = 1

// maxSourceBodySize bounds the JSON source payload to protect against a
// misconfigured URL pointing at a huge file.
const maxSourceBodySize = 1 << 20 // 1 MiB

type sourceDocument struct {
	Version   int            `json:"version"`
	UpdatedAt string         `json:"updated_at"`
	Mirrors   []sourceMirror `json:"mirrors"`
}

type sourceMirror struct {
	URL  string `json:"url"`
	Note string `json:"note"`
}

// FetchCandidates downloads the mirror candidate list, trying each source URL
// in order until one yields a valid document. The returned URLs keep their
// scheme (http:// entries stay plain HTTP), are trimmed and deduplicated in
// document order. An error is returned only when every source fails.
func FetchCandidates(ctx context.Context, sourceURLs []string, timeout time.Duration) ([]string, error) {
	if len(sourceURLs) == 0 {
		return nil, fmt.Errorf("no mirror source url configured")
	}
	client := &http.Client{Timeout: timeout}
	var lastErr error
	for _, sourceURL := range sourceURLs {
		candidates, err := fetchOneSource(ctx, client, sourceURL)
		if err != nil {
			logrus.Warnf("fetch mirror candidates from %s failure: %v", sourceURL, err)
			lastErr = err
			continue
		}
		return candidates, nil
	}
	return nil, fmt.Errorf("all mirror sources failed: %w", lastErr)
}

func fetchOneSource(ctx context.Context, client *http.Client, sourceURL string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request source: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSourceBodySize))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	var doc sourceDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse mirrors json: %w", err)
	}
	if doc.Version != sourceSchemaVersion {
		return nil, fmt.Errorf("unsupported mirrors schema version %d", doc.Version)
	}
	candidates := dedupeMirrorURLs(doc.Mirrors)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("mirrors json contains no usable url")
	}
	return candidates, nil
}

func dedupeMirrorURLs(mirrors []sourceMirror) []string {
	seen := make(map[string]struct{}, len(mirrors))
	result := make([]string, 0, len(mirrors))
	for _, m := range mirrors {
		u := strings.TrimSpace(m.URL)
		if u == "" {
			continue
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		result = append(result, u)
	}
	return result
}
