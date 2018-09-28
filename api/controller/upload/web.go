package upload

import (
	"net/http"
	httputil "github.com/goodrain/rainbond/util/http"
)

type Storage struct {
	output    string
	verbosity int
}

func (s *Storage) StorageDir() string {
	return s.output
}

func NewStorage(rootDir string) *Storage {
	return &Storage{output: rootDir}
}


// UploadHandler is the endpoint for uploading and storing files.
func (s *Storage) UploadHandler(w http.ResponseWriter, r *http.Request) {

	// Performs the processing of writing data into chunk files.
	files, err := process(r, s.StorageDir())

	if err == incomplete {
		httputil.ReturnSuccess(r, w, nil)
		return
	}
	if err != nil {
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}

	data := make([]map[string]interface{}, 0)

	for _, file := range files {
		attachment, err := create(s.StorageDir(), file, true)
		if err != nil {
			httputil.ReturnError(r, w, 500, err.Error())
			return
		}
		data = append(data, attachment.ToJson())
	}
	httputil.ReturnSuccess(r, w, data)
}
