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

import (
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	//KeyPrefix request path prefix
	KeyPrefix string = "Prefix"
	//KeyHeaders request http headers
	KeyHeaders string = "Headers"
	//KeyDomains request domains
	KeyDomains string = "Domains"
	//KeyMaxConnections The maximum number of connections that Envoy will make to the upstream cluster. If not specified, the default is 1024.
	KeyMaxConnections string = "MaxConnections"
	//KeyMaxPendingRequests The maximum number of pending requests that Envoy will allow to the upstream cluster. If not specified, the default is 1024
	KeyMaxPendingRequests string = "MaxPendingRequests"
	//KeyMaxRequests The maximum number of parallel requests that Envoy will make to the upstream cluster. If not specified, the default is 1024.
	KeyMaxRequests string = "MaxRequests"
	//KeyMaxActiveRetries  The maximum number of parallel retries that Envoy will allow to the upstream cluster. If not specified, the default is 3.
	KeyMaxActiveRetries string = "MaxActiveRetries"
	//KeyUpStream upStream
	KeyUpStream string = "upStream"
	//KeyDownStream downStream
	KeyDownStream string = "downStream"
	//KeyWeight WEIGHT
	KeyWeight string = "Weight"
	//KeyWeightModel MODEL_WEIGHT
	KeyWeightModel string = "weight_model"
	//KeyPrefixModel MODEL_PREFIX
	KeyPrefixModel string = "prefix_model"
	//KeyIntervalMS IntervalMS key
	KeyIntervalMS string = "IntervalMS"
	//KeyConsecutiveErrors ConsecutiveErrors key
	KeyConsecutiveErrors string = "ConsecutiveErrors"
	//KeyBaseEjectionTimeMS BaseEjectionTimeMS key
	KeyBaseEjectionTimeMS string = "BaseEjectionTimeMS"
	//KeyMaxEjectionPercent MaxEjectionPercent key
	KeyMaxEjectionPercent string = "MaxEjectionPercent"
)

// GetOptionValues get value from options
// if not exist,return default value
func GetOptionValues(kind string, sr map[string]interface{}) interface{} {
	switch kind {
	case KeyPrefix:
		if prefix, ok := sr[KeyPrefix]; ok {
			return prefix
		}
		return "/"
	case KeyMaxConnections:
		if circuit, ok := sr[KeyMaxConnections]; ok {
			cc, err := strconv.Atoi(circuit.(string))
			if err != nil {
				logrus.Errorf("strcon circuit error")
				return 1024
			}
			return cc
		}
		return 1024
	case KeyMaxRequests:
		if maxRequest, ok := sr[KeyMaxRequests]; ok {
			mrt, err := strconv.Atoi(maxRequest.(string))
			if err != nil {
				logrus.Errorf("strcon max request error")
				return 1024
			}
			return mrt
		}
		return 1024
	case KeyMaxPendingRequests:
		if maxPendingRequests, ok := sr[KeyMaxPendingRequests]; ok {
			mpr, err := strconv.Atoi(maxPendingRequests.(string))
			if err != nil {
				logrus.Errorf("strcon max pending request error")
				return 1024
			}
			return mpr
		}
		return 1024
	case KeyMaxActiveRetries:
		if maxRetries, ok := sr[KeyMaxActiveRetries]; ok {
			mxr, err := strconv.Atoi(maxRetries.(string))
			if err != nil {
				logrus.Errorf("strcon max retry error")
				return 3
			}
			return mxr
		}
		return 3
	case KeyHeaders:
		var np []Header
		if headers, ok := sr[KeyHeaders]; ok {
			parents := strings.Split(headers.(string), ";")
			for _, h := range parents {
				headers := strings.Split(h, ":")
				//has_header:no 默认
				if len(headers) == 2 {
					if headers[0] == "has_header" && headers[1] == "no" {
						continue
					}
					ph := Header{
						Name:  headers[0],
						Value: headers[1],
					}
					np = append(np, ph)
				}
			}
		}
		return np
	case KeyDomains:
		if domain, ok := sr[KeyDomains]; ok {
			if strings.Contains(domain.(string), ",") {
				mm := strings.Split(domain.(string), ",")
				return mm
			}
			return []string{domain.(string)}
		}
		return []string{"*"}
	case KeyWeight:
		if weight, ok := sr[KeyWeight]; ok {
			w, err := strconv.Atoi(weight.(string))
			if err != nil {
				return 100
			}
			return w
		}
		return 100
	case KeyIntervalMS:
		if in, ok := sr[KeyIntervalMS]; ok {
			w, err := strconv.Atoi(in.(string))
			if err != nil {
				return int64(10000)
			}
			return int64(w)
		}
		return int64(10000)
	case KeyConsecutiveErrors:
		if in, ok := sr[KeyConsecutiveErrors]; ok {
			w, err := strconv.Atoi(in.(string))
			if err != nil {
				return 5
			}
			return w
		}
		return 5
	case KeyBaseEjectionTimeMS:
		if in, ok := sr[KeyBaseEjectionTimeMS]; ok {
			w, err := strconv.Atoi(in.(string))
			if err != nil {
				return int64(30000)
			}
			return int64(w)
		}
		return int64(30000)
	case KeyMaxEjectionPercent:
		if in, ok := sr[KeyMaxEjectionPercent]; ok {
			w, err := strconv.Atoi(in.(string))
			if err != nil || w > 100 {
				return 10
			}
			return w
		}
		return 10
	default:
		return nil
	}
}
