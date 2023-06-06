package envutil

// GetMemoryType returns the memory type based on the given memory size.
func GetMemoryType(memorySize int) string {
	memoryType := "small"
	if v, ok := memoryLabels[memorySize]; ok {
		memoryType = v
	}
	return memoryType
}

var memoryLabels = map[int]string{
	128:   "micro",
	256:   "small",
	512:   "medium",
	1024:  "large",
	2048:  "2xlarge",
	4096:  "4xlarge",
	8192:  "8xlarge",
	16384: "16xlarge",
	32768: "32xlarge",
	65536: "64xlarge",
}


// IsCustomMemory -
func IsCustomMemory(memory int) bool {
	if _, ok := memoryLabels[memory]; ok {
		return false
	}
	return true
}
