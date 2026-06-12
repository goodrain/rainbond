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

package sources

import "strings"

// mergeMirrors combines the manually configured mirror list (REGISTRY_MIRRORS,
// always first so operator intent wins) with the dynamically discovered one.
// Entries are deduplicated by host, ignoring the scheme, so a manual http://
// override is not shadowed by the same host from the dynamic list.
func mergeMirrors(manual, dynamic []string) []string {
	seen := make(map[string]struct{}, len(manual)+len(dynamic))
	result := make([]string, 0, len(manual)+len(dynamic))
	for _, m := range append(append([]string{}, manual...), dynamic...) {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		host := strings.TrimPrefix(strings.TrimPrefix(m, "https://"), "http://")
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		result = append(result, m)
	}
	return result
}
