package savers

/****************
 * Date: 30/05/16
 * Time: 6:46 PM
 ***************/

import (
	"github.com/stretchr/testify/assert"
	"github.com/twinj/uuid"
	"os"
	"path"
	"runtime"
	"testing"
	"time"
)

const (
	saveDuration = 3
)

func setupFileSystemStateSaver(pPath string, pReport bool) *FileSystemSaver {
	return &FileSystemSaver{Path: pPath, Report: pReport, Duration: saveDuration * time.Second}
}

// Tests that the schedule is run on the timeDuration
func TestFileSystemSaver_SaveSchedule(t *testing.T) {
	saver := setupFileSystemStateSaver(path.Join("github.com.twinj.uuid.generator-"+uuid.NewV4().String()+".gob"), true)

	// Read is always called first
	saver.Read()

	count := 0

	past := time.Now()
	for count == 10 {
		if uuid.Now() > saver.Timestamp {
			time.Sleep(saver.Duration)
			count++
		}
		store := uuid.Store{Timestamp: uuid.Now(), Sequence: 3, Node: []byte{0xff, 0xaa, 0x11}}
		saver.Save(store)
	}
	d := time.Since(past)

	assert.Equal(t, int(d/saver.Duration), count, "Should be as many saves as second increments")
}

func TestFileSystemSaver_Read(t *testing.T) {
	paths := []string{
		path.Join(os.TempDir(), "test", "github.com.twinj.uuid.generator-"+uuid.NewV4().String()+".gob"),
		path.Join(os.TempDir(), "github.com.twinj.uuid.generator-"+uuid.NewV4().String()+".gob"),
		path.Join("github.com.twinj.uuid.generator-" + uuid.NewV4().String() + ".gob"),
		path.Join("/github.com.twinj.uuid.generator-" + uuid.NewV4().String() + ".gob"),
		path.Join("/github.com.twinj.uuid.generator-" + uuid.NewV4().String()),
		path.Join("/generator-" + uuid.NewV4().String()),
	}

	for i := range paths {

		saver := setupFileSystemStateSaver(paths[i], true)
		err, _ := saver.Read()

		assert.NoError(t, err, "Path failure %d %s", i, paths[i])
	}

	// Empty path
	saver := setupFileSystemStateSaver("", true)
	err, _ := saver.Read()

	assert.Error(t, err, "Expect path failure")

	// No permissions
	if runtime.GOOS == "windows" {
		saver := setupFileSystemStateSaver("C:/windows/generator-delete.gob", true)
		err, _ := saver.Read()
		assert.Error(t, err, "Expect path failure")

		saver = setupFileSystemStateSaver(path.Join("C:/windows", uuid.NewV4().String(), "generator-delete.gob"), true)
		err, _ = saver.Read()
		assert.Error(t, err, "Expect path failure")
	}

	// No permissions
	if runtime.GOOS == "linux" {
		saver := setupFileSystemStateSaver("/root/generator-delete.gob", true)
		err, _ := saver.Read()
		assert.Error(t, err, "Expect path failure")

		saver = setupFileSystemStateSaver(path.Join("/root", uuid.NewV4().String(), "generator-delete.gob"), true)
		err, _ = saver.Read()
		assert.Error(t, err, "Expect path failure")
	}

}

func TestFileSystemSaver_Save(t *testing.T) {

	saver := setupFileSystemStateSaver(path.Join("github.com.twinj.uuid.generator-"+uuid.NewV4().String()+".gob"), true)

	// Read is always called first
	saver.Read()

	store := uuid.Store{Timestamp: 1, Sequence: 2, Node: []byte{0xff, 0xaa, 0x33, 0x44, 0x55, 0x66}}
	saver.Save(store)

	saver = setupFileSystemStateSaver(path.Join("/generator-"+uuid.NewV4().String()+".gob"), false)

	// Read is always called first
	saver.Read()

	store = uuid.Store{Timestamp: 1, Sequence: 2, Node: []byte{0xff, 0xaa, 0x33, 0x44, 0x55, 0x66}}
	saver.Save(store)
}

func TestFileSystemSaver_SaveAndRead(t *testing.T) {

	saver := setupFileSystemStateSaver(path.Join("github.com.twinj.uuid.generator-"+uuid.NewV4().String()+".gob"), true)

	// Read is always called first
	saver.Read()

	store := uuid.Store{Timestamp: 1, Sequence: 2, Node: []byte{0xff, 0xaa, 0x33, 0x44, 0x55, 0x66}}
	saver.Save(store)

	_, saved := saver.Read()

	assert.Equal(t, store.Timestamp, saved.Timestamp)
	assert.Equal(t, store.Sequence, saved.Sequence)
	assert.Equal(t, store.Node, saved.Node)

}
