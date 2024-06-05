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

// CreatOutlierDetection  create outlierDetection
// https://www.envoyproxy.io/docs/envoy/latest/api-v1/cluster_manager/cluster_outlier_detection#config-cluster-manager-cluster-outlier-detection
func CreatOutlierDetection(options map[string]interface{}) *OutlierDetection {
	if _, ok := options[KeyMaxConnections]; !ok {
		return nil
	}
	var od OutlierDetection
	od.ConsecutiveErrors = GetOptionValues(KeyConsecutiveErrors, options).(int)
	od.IntervalMS = GetOptionValues(KeyIntervalMS, options).(int64)
	od.BaseEjectionTimeMS = GetOptionValues(KeyBaseEjectionTimeMS, options).(int64)
	od.MaxEjectionPercent = GetOptionValues(KeyMaxEjectionPercent, options).(int)
	return &od
}

// CreateCircuitBreaker create circuitBreaker
// https://www.envoyproxy.io/docs/envoy/latest/api-v1/cluster_manager/cluster_circuit_breakers#config-cluster-manager-cluster-circuit-breakers-v1
func CreateCircuitBreaker(options map[string]interface{}) *CircuitBreaker {
	if _, ok := options[KeyMaxConnections]; !ok {
		return nil
	}
	var cb CircuitBreaker
	cb.Default.MaxConnections = GetOptionValues(KeyMaxConnections, options).(int)
	cb.Default.MaxRequests = GetOptionValues(KeyMaxRequests, options).(int)
	cb.Default.MaxRetries = GetOptionValues(KeyMaxActiveRetries, options).(int)
	cb.Default.MaxPendingRequests = GetOptionValues(KeyMaxPendingRequests, options).(int)
	return &cb
}
