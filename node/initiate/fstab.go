package initiate

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const ()

// Fstab represents a /etc/fstab file.
type Fstab struct {
	Path  string
	Lines []*fstabLine
}

func NewFstab(path string) (*Fstab, error) {
	f := &Fstab{Path: path}
	if err := f.Load(); err != nil {
		return f, err
	}
	return f, nil
}

// Load loads the contents of / etc / fstab into Fstab
func (f *Fstab) Load() (err error) {
	file, err := os.Open(f.Path)
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := newFstabLine(scanner.Text())
		f.Lines = append(f.Lines, line)
	}
	if err = scanner.Err(); err != nil {
		return
	}

	return
}

func (f *Fstab) Add(raw string) {
	fstabLine := newFstabLine(raw)
	f.Lines = append(f.Lines, fstabLine)
}

func (f *Fstab) CheckIfExist(raw string) bool {
	for _, line := range f.Lines {
		if line.Raw == raw {
			return true
		}
	}
	return false
}

func (f *Fstab) AddIfNotExist(raw string) {
	if f.CheckIfExist(raw) {
		return
	}
	f.Add(raw)
	return
}

func (f *Fstab) Flush() error {
	file, err := os.Create(f.Path)
	if err != nil {
		return err
	}

	w := bufio.NewWriter(file)

	for _, line := range f.Lines {
		_, _ = fmt.Fprintf(w, "%s%s", line.Raw, eol)
	}

	err = w.Flush()
	if err != nil {
		return err
	}

	return f.Load()
}

type fstabLine struct {
	Raw        string
	FileSystem string
	MountPoint string
	Type       string
	Options    string
	Dump       int
	Pass       int
	Err        error
}

func newFstabLine(raw string) (f *fstabLine) {
	f = &fstabLine{Raw: raw}
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return
	}

	if f.isComment() {
		return
	}

	if len(fields) != 6 {
		f.Err = errors.New(fmt.Sprintf("Bad fstab line: %q", raw))
		return
	}

	f.FileSystem = fields[0]
	f.MountPoint = fields[1]
	f.Type = fields[2]
	f.Options = fields[3]
	f.Dump, _ = strconv.Atoi(fields[4])
	f.Pass, _ = strconv.Atoi(fields[5])
	return
}

func (f *fstabLine) isComment() bool {
	line := strings.TrimSpace(f.Raw)
	isComment := strings.HasPrefix(line, commentChar)
	return isComment
}
