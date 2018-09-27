package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestUploadBinary(t *testing.T) {
	assert := assert.New(t)

	req, _ := http.NewRequest("POST", "/files", nil)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-File", "./dummy/bin-data")
	req.Header.Set("Content-Disposition", `attachment; filename="basta.png"`)
	req.AddCookie(&http.Cookie{Name: "coquelicot", Value: "abcdef"})

	r := gin.Default()

	r.POST("/files", CreateAttachment)
	rw := httptest.NewRecorder()

	r.ServeHTTP(rw, req)
	assert.Equal(http.StatusCreated, rw.Code)

	//var d map[string]interface{}
	//json.Unmarshal(rw.Body.Bytes(), &d)

	//t.Logf("json decode: %+v", d)
}

func TestGetConvertParams(t *testing.T) {
	assert := assert.New(t)
	req, _ := http.NewRequest("POST", `/files?converts={"pic":"120x90"}`, nil)

	convert, err := GetConvertParams(req)

	assert.Nil(err)
	assert.Equal("120x90", convert["pic"])
}
