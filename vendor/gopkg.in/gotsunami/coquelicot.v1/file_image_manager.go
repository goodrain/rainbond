package coquelicot

import (
	"fmt"
	"os/exec"
)

type fileImageManager struct {
	*fileDefaultManager
	Width     int
	Height    int
	thumbnail bool // Add resized version with ImageMagick
}

// Save version from original with convert command-line tool.
func (fim *fileImageManager) convert(src string, convert string) error {
	if !fim.thumbnail {
		// Raw copy
		return fim.rawCopy(src, convert)
	}
	err := convertImage(src, fim.Filepath(), convert)
	if err != nil {
		return err
	}

	fim.Width, fim.Height, fim.Size, err = identifyImageSizes(fim.Filepath())
	if err != nil {
		return err
	}

	return nil
}

func (fim *fileImageManager) ToJson() map[string]interface{} {
	return map[string]interface{}{
		"url":      fim.Url(),
		"filename": fim.Filename,
		"size":     fim.Size,
		"width":    fim.Width,
		"height":   fim.Height,
	}

}

func convertImage(src, dest, convert string) error {
	args := []string{src, "-strip"}
	if convert != "" {
		cv := []string{"-resize", convert + "^", "-gravity", "center", "-extent", convert}
		args = append(args, cv...)
	}
	args = append(args, dest)

	out, err := exec.Command("convert", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("Error move original: %s, %s", err, string(out))
	}

	return nil
}

func identifyImageSizes(filepath string) (int, int, int64, error) {
	cmd := exec.Command("identify", "-format", `"%w:%h:%b"`, filepath)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("Identify Sizes: %s; detail: %s", err, string(out))
	}

	var w, h int
	var s int64

	fmt.Sscanf(string(out), `"%d:%d:%dB"`, &w, &h, &s)

	return w, h, s, nil
}
