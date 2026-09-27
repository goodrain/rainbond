package registryproxy

import (
	"os/exec"
	"strconv"
	"syscall"
)

func gcDescriptorPath(fd int) string { return "/proc/self/fd/" + strconv.Itoa(fd) }
func configureGCProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
}
