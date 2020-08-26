// +build darwin

package term

// GetWinsize returns the window size based on the specified file descriptor.
func GetWinsize(fd uintptr) (*Winsize, error) {
	ws := &Winsize{Height: 128, Width: 128, x: 0, y: 0}
	return ws, nil
}

// SetWinsize tries to set the specified window size for the specified file descriptor.
func SetWinsize(fd uintptr, ws *Winsize) error {
	return nil
}
