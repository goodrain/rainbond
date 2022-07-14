package util

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func FormatPath(s string) string {
	log.Println("runtime.GOOS:", runtime.GOOS)
	switch runtime.GOOS {
	case "windows":
		return strings.Replace(s, "/", "\\", -1)
	case "darwin", "linux":
		return strings.Replace(s, "\\", "/", -1)
	default:
		logrus.Info("only support linux,windows,darwin, but os is " + runtime.GOOS)
		return s
	}
}

func MoveDir(src string, dest string) error {
	src = FormatPath(src)
	dest = FormatPath(dest)
	logrus.Info("src", src)
	logrus.Info("dest", dest)

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("xcopy", src, dest, "/I", "/E")
	case "darwin", "linux":
		cmd = exec.Command("cp", "-R", src, dest)
	}
	outPut, err := cmd.Output()
	if err != nil {
		logrus.Errorf("Output error: %s", err.Error())
		return err
	}
	fmt.Println(outPut)
	if err := os.RemoveAll(src); err != nil {
		logrus.Errorf("remove oldpath error: %s", err.Error())
		return err
	}
	return nil
}
