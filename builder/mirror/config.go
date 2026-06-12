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
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// defaultSourceURL is the goodrain-maintained mirrors.json served through the
// jsDelivr CDN, which is reachable from mainland-China clusters.
const defaultSourceURL = "https://cdn.jsdelivr.net/gh/goodrain/docker-mirrors@main/mirrors.json"

const (
	defaultRefreshInterval = 6 * time.Hour
	defaultMaxCount        = 3
)

// Config controls the dynamic mirror manager. All fields come from builder
// environment variables; zero-configuration deployments get safe defaults.
type Config struct {
	// Enabled gates the whole feature (DYNAMIC_REGISTRY_MIRRORS, default true).
	Enabled bool
	// SourceURLs are tried in order when fetching candidates (MIRROR_SOURCE_URLS).
	SourceURLs []string
	// RefreshInterval is the period between refresh runs (MIRROR_REFRESH_INTERVAL).
	RefreshInterval time.Duration
	// MaxCount caps how many alive mirrors are kept (MIRROR_MAX_COUNT).
	MaxCount int
}

// LoadConfig builds a Config from the given env lookup (usually os.Getenv).
// Invalid values fall back to defaults with a warning instead of failing the
// builder startup.
func LoadConfig(getenv func(string) string) Config {
	cfg := Config{
		Enabled:         true,
		SourceURLs:      []string{defaultSourceURL},
		RefreshInterval: defaultRefreshInterval,
		MaxCount:        defaultMaxCount,
	}
	if raw := getenv("DYNAMIC_REGISTRY_MIRRORS"); raw != "" {
		cfg.Enabled = strings.EqualFold(raw, "true")
	}
	if raw := getenv("MIRROR_SOURCE_URLS"); raw != "" {
		urls := make([]string, 0)
		for _, u := range strings.Split(raw, ",") {
			if u = strings.TrimSpace(u); u != "" {
				urls = append(urls, u)
			}
		}
		if len(urls) > 0 {
			cfg.SourceURLs = urls
		}
	}
	if raw := getenv("MIRROR_REFRESH_INTERVAL"); raw != "" {
		if interval, err := time.ParseDuration(raw); err == nil && interval > 0 {
			cfg.RefreshInterval = interval
		} else {
			logrus.Warnf("invalid MIRROR_REFRESH_INTERVAL %q, using default %v", raw, defaultRefreshInterval)
		}
	}
	if raw := getenv("MIRROR_MAX_COUNT"); raw != "" {
		if count, err := strconv.Atoi(raw); err == nil && count > 0 {
			cfg.MaxCount = count
		} else {
			logrus.Warnf("invalid MIRROR_MAX_COUNT %q, using default %d", raw, defaultMaxCount)
		}
	}
	return cfg
}
