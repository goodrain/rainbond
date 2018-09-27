package coquelicot

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"code.google.com/p/go-uuid/uuid"

	"github.com/stretchr/testify/assert"
)

const dummy = "bin/coquelicot/dummy"

func TestUploadMultipart(t *testing.T) {
	assert := assert.New(t)

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)

	if err := writeMPbody(dummy+"/32509211_news_bigpic.jpg", mw); err != nil {
		assert.Error(err)
	}
	if err := writeMPbody(dummy+"/kino.jpg", mw); err != nil {
		assert.Error(err)
	}

	mw.Close()

	req, _ := http.NewRequest("POST", "/files", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "coquelicot", Value: "abcdef"})

	files, err := process(req, dummy+"/root_storage")
	assert.Nil(err)
	assert.Equal("kino.jpg", files[1].Filename)
	assert.Equal("image", files[1].BaseMime)

}
func TestUploadBinary(t *testing.T) {
	assert := assert.New(t)

	req, _ := http.NewRequest("POST", "/files", nil)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-File", dummy+"/bin-data")
	req.Header.Set("Content-Disposition", `attachment; filename="basta.png"`)
	req.AddCookie(&http.Cookie{Name: "coquelicot", Value: "abcdef"})

	files, err := process(req, dummy+"/root_storage")
	assert.Nil(err)
	assert.Equal("basta.png", files[0].Filename)
	assert.Equal("image", files[0].BaseMime)

}

func TestUploadChunked(t *testing.T) {
	assert := assert.New(t)
	storage := dummy + "/root_storage"
	fname := dummy + "/kino.jpg"
	f, _ := os.Open(fname)
	defer f.Close()

	cookie := &http.Cookie{Name: "coquelicot", Value: uuid.New()}

	req := createChunkRequest(f, 0, 24999)
	req.AddCookie(cookie)
	files, err := process(req, storage)
	assert.Equal(incomplete, err)
	assert.Equal(25000, int(files[0].Size))

	req = createChunkRequest(f, 25000, 49999)
	req.AddCookie(cookie)
	files, err = process(req, storage)
	assert.Equal(incomplete, err)
	assert.Equal(50000, int(files[0].Size))

	req = createChunkRequest(f, 50000, 52096)
	req.AddCookie(cookie)
	files, err = process(req, storage)
	assert.Nil(err)
	assert.Equal(52097, int(files[0].Size))
	assert.Equal("kino.jpg", files[0].Filename)
}

func createChunkRequest(f *os.File, start int64, end int64) *http.Request {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fi, _ := f.Stat()
	fw, _ := mw.CreateFormFile("files[]", fi.Name())

	io.CopyN(fw, f, end-start+1)
	mw.Close()

	req, _ := http.NewRequest("POST", "/files", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Content-Disposition", `attachment; filename="`+fi.Name()+`"`)
	req.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fi.Size()))

	return req
}

func TestTempFileChunks(t *testing.T) {
	assert := assert.New(t)

	file, err := tempFileChunks(0, dummy+"/root_storage", "abcdef", "kino.jpg")
	assert.Nil(err)
	assert.NotNil(file)
}

func writeMPbody(fname string, mw *multipart.Writer) error {
	fw, _ := mw.CreateFormFile("files[]", filepath.Base(fname))
	f, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(fw, f)
	if err != nil {
		return err
	}

	return nil
}
