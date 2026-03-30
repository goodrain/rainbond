package sources

import "testing"

// capability_id: rainbond.source-sftp.port-parse
func TestParseSFTPPort(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{input: "20012", want: 20012},
		{input: "", want: 21},
		{input: "not-a-number", want: 21},
	}

	for _, tt := range tests {
		if got := parseSFTPPort(tt.input); got != tt.want {
			t.Fatalf("parseSFTPPort(%q)=%d, want %d", tt.input, got, tt.want)
		}
	}
}

// capability_id: rainbond.source-sftp.close-safe
func TestSFTPClientCloseZeroValue(t *testing.T) {
	client := &SFTPClient{}
	client.Close()
}
