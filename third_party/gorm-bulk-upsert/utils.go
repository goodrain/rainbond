package gormbulkups

import "sort"

func splitObjects(objArr []interface{}, size int) [][]interface{} {
	var chunkSet [][]interface{}
	var chunk []interface{}

	for len(objArr) > size {
		chunk, objArr = objArr[:size], objArr[size:]
		chunkSet = append(chunkSet, chunk)
	}
	if len(objArr) > 0 {
		chunkSet = append(chunkSet, objArr[:])
	}
	return chunkSet
}

func sortedKeys(val map[string]interface{}) []string {
	keys := make([]string, 0, len(val))
	for key := range val {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func containString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
