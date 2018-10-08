package upload

import (
	"os"
)

// attachment contain info about directory, base mime type and all files saved.
type attachment struct {
	originalFile *originalFile
	Dir          *dirManager
	Versions     map[string]fileManager
}

func create(storage string, ofile *originalFile, delChunk bool) (*attachment, error) {
	dm, err := createDir(storage, ofile.BaseMime)
	if err != nil {
		return nil, err
	}

	at := &attachment{
		originalFile: ofile,
		Dir:          dm,
		Versions:     make(map[string]fileManager),
	}


	makeVersion := func(a *attachment, version, convert string) error {
		fm, err := at.createVersion(version)
		if err != nil {
			return err
		}
		at.Versions[version] = fm
		return nil
	}

	if err := makeVersion(at, "original", ""); err != nil {
		return nil, err
	}

	if delChunk {
		return at, os.Remove(at.originalFile.Filepath)
	}
	return at, nil
}

// Directly save single version and return fileManager.
func (attachment *attachment) createVersion(version string) (fileManager, error) {
	fm := newFileManager(attachment.Dir, version)
	fm.SetFilename(attachment.originalFile)

	if err := fm.convert(attachment.originalFile.Filepath); err != nil {
		return nil, err
	}

	return fm, nil
}

func (attachment *attachment) ToJson() map[string]interface{} {
	data := make(map[string]interface{})
	data["type"] = attachment.originalFile.BaseMime
	data["dir"] = attachment.Dir.Path
	data["name"] = attachment.originalFile.Filename
	versions := make(map[string]interface{})
	for version, fm := range attachment.Versions {
		versions[version] = fm.ToJson()
	}
	data["versions"] = versions

	return data
}
