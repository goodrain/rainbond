package registryproxy

import (
	"os/exec"
	"strconv"
)

func gcDescriptorPath(fd int) string       { return "/dev/fd/" + strconv.Itoa(fd) }
func configureGCProcess(command *exec.Cmd) {}
