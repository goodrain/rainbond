// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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

package v1

import "time"

const (
	// StatHealthy -
	StatHealthy string = "healthy"
	// StatUnhealthy -
	StatUnhealthy string = "unhealthy"
	// StatDeath -
	StatDeath string = "death"
)

// Service Service
type Service struct {
	Sid           string  `json:"service_id"`
	Name          string  `json:"name"`
	ServiceHealth *Health `json:"health"`
	Disable       bool    `json:"disable"`
}

// Equal check if the left service(l) is equal to the right service(r)
func (l *Service) Equal(r *Service) bool {
	if l == r {
		return true
	}
	if l.Sid != r.Sid {
		return false
	}
	if l.Name != r.Name {
		return false
	}
	if l.Disable != r.Disable {
		return false
	}
	if !l.ServiceHealth.Equal(r.ServiceHealth) {
		return false
	}
	return true
}

//Health ServiceHealth
type Health struct {
	Name             string `json:"name"`
	Model            string `json:"model"`
	IP               string `json:"ip"`
	Port             int    `json:"port"`
	Address          string `json:"address"`
	TimeInterval     int    `json:"time_interval"`
	MaxErrorsNum     int    `json:"max_errors_num"`
	MaxTimeoutSecond int    `json:"max_timeout"`
}

// Equal check if the left health(l) is equal to the right health(r)
func (l *Health) Equal(r *Health) bool {
	if l == r {
		return true
	}
	if l.Name != r.Name {
		return false
	}
	if l.Model != r.Model {
		return false
	}
	if l.IP != r.IP {
		return false
	}
	if l.Port != r.Port {
		return false
	}
	if l.Address != r.Address {
		return false
	}
	if l.TimeInterval != r.TimeInterval {
		return false
	}
	if l.MaxErrorsNum != r.MaxErrorsNum {
		return false
	}
	return true
}

//HealthStatus health status
type HealthStatus struct {
	Name           string        `json:"name"`
	Status         string        `json:"status"`
	ErrorNumber    int           `json:"error_number"`
	ErrorDuration  time.Duration `json:"error_duration"`
	StartErrorTime time.Time     `json:"start_error_time"`
	Info           string        `json:"info"`
	LastStatus     string        `json:"last_status"`
	StatusChange   bool          `json:"status_change"`
}
