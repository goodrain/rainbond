package upload

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Error incomplete returned by uploader when loaded non-last chunk.
var incomplete = errors.New("incomplete")

// Structure describes the state of the original file.
type originalFile struct {
	BaseMime string
	Filepath string
	Filename string
	Size     int64
}

func (ofile *originalFile) Ext() string {
	return strings.ToLower(filepath.Ext(ofile.Filename))
}


func process(req *http.Request, storage string) ([]*originalFile, error) {
	meta, err := parseMeta(req)
	if err != nil {
		return nil, err
	}

	body, err := newBody(req.Body)
	if err != nil {
		return nil, err
	}
	up := &uploader{Root: storage, Meta: meta, body: body}

	files, err := up.SaveFiles()
	if err == incomplete {
		return files, err
	}
	if err != nil {
		return nil, err
	}

	return files, nil
}

// Upload manager.
type uploader struct {
	Root string
	Meta *meta
	body *body
}

// Function SaveFiles sequentially loads the original files or chunk's.
func (up *uploader) SaveFiles() ([]*originalFile, error) {
	files := make([]*originalFile, 0)
	for {
		ofile, err := up.SaveFile()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		files = append(files, ofile)
	}

	return files, nil
}

// Function loads one or download the original file chunk.
// Asks for the starting position in the body of the request to read the next file.
// Asks for a temporary file.
// Writes data from the request body into a temporary file.
// Specifies the size of the resulting temporary file.
// If the query specified header Content-Range,
// and the size of the resulting file does not match, it returns an error incomplete.
// Otherwise, defines the basic mime type, and returns the original file.
func (up *uploader) SaveFile() (*originalFile, error) {
	body, filename, err := up.Reader()
	if err != nil {
		return nil, err
	}

	temp_file, err := up.tempFile()
	if err != nil {
		return nil, err
	}
	defer temp_file.Close()

	if err = up.Write(temp_file, body); err != nil {
		return nil, err
	}

	fi, err := temp_file.Stat()
	if err != nil {
		return nil, err
	}

	ofile := &originalFile{Filename: filename, Filepath: temp_file.Name(), Size: fi.Size()}

	ofile.BaseMime, err = identifyMime(ofile.Filepath)
	if err != nil {
		return nil, err
	}

	return ofile, nil
}

// Returns the reader to read the file or chunk of request body and the original file name.
// If the request header Content-Type is multipart/form-data, returns the next copy part.
// If all of part read the case of binary loading read the request body, an error is returned io.EOF.
func (up *uploader) Reader() (io.Reader, string, error) {
	if up.Meta.MediaType == "multipart/form-data" {
		if up.body.MR == nil {
			up.body.MR = multipart.NewReader(up.body.body, up.Meta.Boundary)
		}
		for {
			part, err := up.body.MR.NextPart()
			if err != nil {
				return nil, "", err
			}
			if part.FormName() == "files[]" {
				return part, part.FileName(), nil
			}
		}
	}

	if !up.body.Available {
		return nil, "", io.EOF
	}

	up.body.Available = false

	return up.body.body, up.Meta.Filename, nil
}

// Returns a temporary file to download the file or resume chunk.
func (up *uploader) tempFile() (*os.File, error) {
	return tempFile()
}

// Returns the newly created temporary file.
func tempFile() (*os.File, error) {
	return ioutil.TempFile(os.TempDir(), "coquelicot")
}

// Returns a temporary file to download chunk.
// To calculate a unique file name used cookie named coquelicot and the original file name.
// File located in the directory chunks storage root directory.
// Before returning the file pointer is shifted by the value of offset,
// in a situation where the pieces are loaded from the second to the last.
func tempFileChunks(offset int64, storage, upload_sid, user_filename string) (*os.File, error) {
	hasher := md5.New()
	hasher.Write([]byte(upload_sid + user_filename))
	filename := hex.EncodeToString(hasher.Sum(nil))

	path := filepath.Join(storage, "chunks")

	err := os.MkdirAll(path, 0755)
	if err != nil {
		return nil, err
	}

	file, err := os.OpenFile(filepath.Join(path, filename), os.O_CREATE|os.O_WRONLY, 0664)
	if err != nil {
		return nil, err
	}

	if _, err = file.Seek(offset, 0); err != nil {
		return nil, err
	}

	return file, nil
}

// The function writes a temporary file value from reader.
func (up *uploader) Write(temp_file *os.File, body io.Reader) error {
	var err error
	_, err = io.Copy(temp_file, body)
	return err
}

// identifyMine gets base mime type.
func identifyMime(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()
	// DetectContentType reads at most the first 512 bytes
	buf := make([]byte, 512)
	_, err = f.Read(buf)
	if err != nil {
		return "", err
	}
	mime := strings.Split(http.DetectContentType(buf), "/")[0]

	return mime, nil
}
