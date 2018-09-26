package coquelicot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateAttachment(t *testing.T) {
	assert := assert.New(t)

	ofile := originalImageFile()
	storage := dummy + "/root_storage"
	converts := map[string]string{"original": "", "thumbnail": "120x80"}

	attachment, err := create(storage, ofile, converts, false)
	assert.Nil(err)
	// Convert option not set
	assert.Equal(len(attachment.Versions), 1)

	data := attachment.ToJson()
	assert.Equal(data["type"], "image")
}

func originalImageFile() *originalFile {
	return &originalFile{
		BaseMime: "image",
		Filepath: dummy + "/32509211_news_bigpic.jpg",
		Filename: "32509211_news_bigpic.jpg",
	}
}

func originalPdfFile() *originalFile {
	return &originalFile{
		BaseMime: "application",
		Filepath: dummy + "/Learning-Go-latest.pdf",
		Filename: "Learning-Go-latest.pdf",
	}
}
