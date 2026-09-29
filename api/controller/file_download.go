package controller

import (
	"net/http"
	"os"
)

func serveDownloadedFile(w http.ResponseWriter, r *http.Request, fileName string) error {
	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}

	w.Header().Set("status", "success")
	w.Header().Set("Content-Disposition", "attachment;filename="+info.Name())
	// ServeFile redirects /index.html to ./, which does not match the download route.
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
	return nil
}
