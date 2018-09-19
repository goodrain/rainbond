package directio_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/ncw/directio"
)

func TestDirectIo(t *testing.T) {
	// Make a temporary file name
	fd, err := ioutil.TempFile("", "direct_io_test")
	if err != nil {
		t.Fatal("Failed to make temp file", err)
	}
	path := fd.Name()
	fd.Close()

	// starting block
	block1 := directio.AlignedBlock(directio.BlockSize)
	for i := 0; i < len(block1); i++ {
		block1[i] = 'A'
	}

	// Write the file
	out, err := directio.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		t.Fatal("Failed to directio.OpenFile for read", err)
	}
	_, err = out.Write(block1)
	if err != nil {
		t.Fatal("Failed to write", err)
	}
	err = out.Close()
	if err != nil {
		t.Fatal("Failed to close writer", err)
	}

	// Read the file
	block2 := directio.AlignedBlock(directio.BlockSize)
	in, err := directio.OpenFile(path, os.O_RDONLY, 0666)
	if err != nil {
		t.Fatal("Failed to directio.OpenFile for write", err)
	}
	_, err = io.ReadFull(in, block2)
	if err != nil {
		t.Fatal("Failed to read", err)
	}
	err = in.Close()
	if err != nil {
		t.Fatal("Failed to close reader", err)
	}

	// Tidy
	err = os.Remove(path)
	if err != nil {
		t.Fatal("Failed to remove temp file", path, err)
	}

	// Compare
	if !bytes.Equal(block1, block2) {
		t.Fatal("Read not the same as written")
	}
}

func TestZeroSizedBlock(t *testing.T) {
	// This should not panic!
	directio.AlignedBlock(0)
}
