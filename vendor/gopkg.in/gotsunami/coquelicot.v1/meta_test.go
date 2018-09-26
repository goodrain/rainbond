package coquelicot

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestparseMeta(t *testing.T) {
	assert := assert.New(t)

	req, _ := http.NewRequest("POST", "/files", nil)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=----Zam1WUeLK7vBj4wN")
	req.Header.Set("Content-Range", "bytes 512000-1023999/1141216")
	req.Header.Set("Content-Disposition", `attachment; filename="picture.jpg"`)
	req.AddCookie(&http.Cookie{Name: "coquelicot", Value: "abcdef"})

	meta, err := parseMeta(req)
	assert.Nil(err)

	assert.Equal(meta.MediaType, "multipart/form-data")
	assert.Equal(meta.Boundary, "----Zam1WUeLK7vBj4wN")

	assert.Equal(meta.Range.Start, int64(512000))
	assert.Equal(meta.Range.End, int64(1023999))
	assert.Equal(meta.Range.Size, int64(1141216))

	assert.Equal(meta.Filename, "picture.jpg")

	assert.Equal(meta.UploadSid, "abcdef")
}
