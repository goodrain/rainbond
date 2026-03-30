package util

import "testing"

// capability_id: rainbond.util.bytes-equality
func TestBytesSliceEqual(t *testing.T) {
	if !BytesSliceEqual([]byte("rainbond"), []byte("rainbond")) {
		t.Fatal("expected equal byte slices")
	}
	if BytesSliceEqual([]byte("rainbond"), []byte("rain")) {
		t.Fatal("did not expect slices with different lengths to be equal")
	}
	if BytesSliceEqual([]byte("rainbond"), []byte("rainb0nd")) {
		t.Fatal("did not expect slices with different content to be equal")
	}
	if BytesSliceEqual(nil, []byte{}) {
		t.Fatal("did not expect nil and empty slice to be equal")
	}
}

// capability_id: rainbond.util.bytes-to-string
func TestToString(t *testing.T) {
	if got := ToString([]byte("rainbond")); got != "rainbond" {
		t.Fatalf("unexpected string: %q", got)
	}
}

// capability_id: rainbond.util.string-to-bytes
func TestToByte(t *testing.T) {
	got := ToByte("rainbond")
	if string(got) != "rainbond" {
		t.Fatalf("unexpected bytes: %q", string(got))
	}
}
