package validation

import (
	"fmt"
	"testing"
)

func TestSplitEndpointAddress(t *testing.T) {
	address := SplitEndpointAddress("http://www.baidu.com")
	fmt.Printf("endpoint: %s, addrss: %s, is IP? %t \n", "http://www.baidu.com", address, len(ValidateEndpointIP(address)) == 0)
	address = SplitEndpointAddress("http://www.baidu.com:443")
	fmt.Printf("endpoint: %s, addrss: %s, is IP? %t \n", "http://www.baidu.com:443", address, len(ValidateEndpointIP(address)) == 0)
	address = SplitEndpointAddress("http://10.211.55.3:7070")
	fmt.Printf("endpoint: %s, addrss: %s, is IP? %t \n", "http://10.211.55.3:7070", address, len(ValidateEndpointIP(address)) == 0)
}
