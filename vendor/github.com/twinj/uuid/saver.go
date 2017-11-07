package uuid

import "fmt"

import ()

// an iterated value to help ensure unique UUID generations values
// across the same domain, server restarts and clock issues
type Sequence uint16

// the last node id setup used by the generator
type Node []byte

// Store is used for storage of UUID generation history to ensure continuous
// running of the UUID generator between restarts and to monitor synchronicity
// while generating new V1 or V2 UUIDs
type Store struct {
	Timestamp
	Sequence
	Node
}

// String returns a string representation of the Store
func (o Store) String() string {
	return fmt.Sprintf("Timestamp[%s]-Sequence[%d]-Node[%x]", o.Timestamp, o.Sequence, o.Node)
}

// Saver is an interface to setup a non volatile store within your system
// if you wish to use V1 and V2 UUIDs based on your node id and a constant time
// it is highly recommended to implement this.
// A default implementation has been provided. FileSystemStorage, the default
// behaviour of the package is to generate random sequences where a Saver is not
// specified.
type Saver interface {
	// Read is run once, use this to setup your UUID state machine
	// Read should also return the UUID state from the non volatile store
	Read() (error, Store)

	// Save saves the state to the non volatile store and is called only if
	Save(Store)
}

// RegisterSaver register's a uuid.Saver implementation to the default package
// uuid.Generator. If you wish to save the generator state, this function must
// be run before any calls to V1 or V2 UUIDs. uuid.RegisterSaver cannot be run
// in conjunction with uuid.Init. You may implement the uuid.Saver interface
// or use the provided uuid.Saver's from the uuid/savers package.
func RegisterSaver(pSaver Saver) {
	generator.Do(func() {
		defer generator.init()
		generator.Lock()
		defer generator.Unlock()
		generator.Saver = pSaver
	})
}
