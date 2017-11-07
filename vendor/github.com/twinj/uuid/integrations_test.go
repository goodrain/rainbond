package uuid_test

import (
	"github.com/stretchr/testify/assert"
	. "github.com/twinj/uuid"
	"testing"
)

func TestInit(t *testing.T) {
	assert.Panics(t, didInitPanic, "Should panic")
}

func didInitPanic() {
	Init()
	Init()
}
