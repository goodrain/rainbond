package savers

/****************
 * Date: 30/05/16
 * Time: 5:48 PM
 ***************/

import (
	"encoding/gob"
	"github.com/twinj/uuid"
	"log"
	"os"
	"path"
	"time"
)

var _ uuid.Saver = &FileSystemSaver{}

// This implements the Saver interface for UUIDs
type FileSystemSaver struct {
	// A file to save the state to
	// Used gob format on uuid.State entity
	file *os.File

	// Preferred location for the store
	Path string

	// Whether to log each save
	Report bool

	// The amount of time between each save call
	time.Duration

	// The next time to save
	uuid.Timestamp
}

func (o *FileSystemSaver) Save(pStore uuid.Store) {

	if pStore.Timestamp >= o.Timestamp {
		err := o.openAndDo(o.encode, &pStore)
		if err == nil {
			if o.Report {
				log.Printf("UUID Saved State Storage: %s", pStore)
			}
		}
		o.Timestamp = pStore.Add(o.Duration)
	}
}

func (o *FileSystemSaver) Read() (err error, store uuid.Store) {
	store = uuid.Store{}
	gob.Register(&uuid.Store{})

	if _, err = os.Stat(o.Path); os.IsNotExist(err) {
		dir, file := path.Split(o.Path)
		if dir == "" || dir == "/" {
			dir = os.TempDir()
		}
		o.Path = path.Join(dir, file)

		err = os.MkdirAll(dir, os.ModeDir|0755)
		if err == nil {
			// If new encode blank store
			err = o.openAndDo(o.encode, &store)
			if err == nil {
				log.Println("uuid.FileSystemSaver created", o.Path)
			}
		}
		log.Println("uuid.FileSystemSaver.Read: error will autogenerate", err)
		return
	}

	o.openAndDo(o.decode, &store)
	return
}

func (o *FileSystemSaver) openAndDo(fDo func(*uuid.Store), pStore *uuid.Store) (err error) {
	o.file, err = os.OpenFile(o.Path, os.O_RDWR|os.O_CREATE, os.ModeExclusive)
	defer o.file.Close()
	if err == nil {
		fDo(pStore)
	} else {
		log.Println("uuid.FileSystemSaver.openAndDo error:", err)
	}
	return
}

func (o *FileSystemSaver) encode(pStore *uuid.Store) {
	// ensure reader state is ready for use
	enc := gob.NewEncoder(o.file)
	// swallow error for encode as its only for cyclic pointers
	enc.Encode(&pStore)
}

func (o *FileSystemSaver) decode(pStore *uuid.Store) {
	// ensure reader state is ready for use
	o.file.Seek(0, 0)
	dec := gob.NewDecoder(o.file)
	// swallow error for encode as its only for cyclic pointers
	dec.Decode(&pStore)
}
