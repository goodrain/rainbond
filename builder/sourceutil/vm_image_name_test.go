package sourceutil

import "testing"

// capability_id: rainbond.vm-build.safe-url-filename
func TestVMImageFileNameFromURL(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{"https://example.invalid/%E4%B9%8C%E7%8F%AD%E5%9B%BE.img?query=/other.iso#fragment/other.raw", "乌班图.img"},
		{"https://example.invalid/folder/disk.RAW.GZ", "disk.RAW.GZ"},
		{"https://example.invalid/磁盘.qcow2", "磁盘.qcow2"},
	} {
		got, err := VMImageFileNameFromURL(tt.input)
		if err != nil || got != tt.want {
			t.Fatalf("filename = %q, %v; want %q", got, err, tt.want)
		}
	}
	for _, suffix := range []string{"", "/", "/folder/", "/%2fimage.img", "/%5cimage.img", "/%00image.img", "/%0aimage.img", "/%20image.img", "/--from=other.img", "/%24HOME.img", "/%60cmd%60.img", "/%252fimage.img", "/%zz.img", "/%E2%80%AEimage.img"} {
		if _, err := VMImageFileNameFromURL("https://example.invalid" + suffix); err == nil {
			t.Fatalf("unsafe filename accepted for suffix %q", suffix)
		}
	}
}
