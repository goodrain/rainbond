package uuid

import (
	"github.com/stretchr/testify/assert"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEpoch(t *testing.T) {
	assert.True(t, gregorianToUNIXOffset == 0x01B21DD213814000)
	assert.True(t, gregorianToUNIXOffset == 122192928000000000)
}

func TestNow(t *testing.T) {
	assert.True(t, Now() > gregorianToUNIXOffset)
}

func TestTimestamp_Add(t *testing.T) {
	now := Now()
	assert.True(t, now.Add(time.Second) == now+Timestamp((time.Second/100)), "The times should be equal")
}

func TestTimestamp_String(t *testing.T) {
	now := Now()
	nowTime := now.Time()
	assert.Equal(t, now.String(), nowTime.String(), "The time strings should be equal")
}

func TestTimestamp_Sub(t *testing.T) {
	now := Now()
	assert.True(t, now.Sub(time.Second) == now-Timestamp((time.Second/100)), "The times should be equal")
}

func TestTimestamp_Time(t *testing.T) {

	assert.True(t, Now().Time().Location() == time.UTC)

}

func TestSpinnerNext(t *testing.T) {
	size := defaultSpinResolution * 1

	spin := spinner{}
	spin.Resolution = defaultSpinResolution

	times := make([]Timestamp, size)

	for i := 0; i < size; i++ {
		times[i] = spin.next()
	}

	for j := size - 1; j >= 0; j-- {
		for k := 0; k < size; k++ {
			if k == j {
				continue
			}
			assert.NotEqual(t, "Timestamps should never be equal", times[j], times[k])
		}
	}

	spin = spinner{
		Count:      defaultSpinResolution - 1,
		Timestamp:  Now(),
		Resolution: 1024,
	}

	for i := 0; i < size; i++ {
		times[i] = spin.next()
	}

	for j := size - 1; j >= 0; j-- {
		for k := 0; k < size; k++ {
			if k == j {
				continue
			}
			assert.NotEqual(t, "Timestamps should never be equal", times[j], times[k])
		}
	}

	spin = spinner{
		Count:      0,
		Timestamp:  Now(),
		Resolution: 1,
	}

	for i := 0; i < size; i++ {
		times[i] = spin.next()
	}

	for j := size - 1; j >= 0; j-- {
		for k := 0; k < size; k++ {
			if k == j {
				continue
			}
			assert.NotEqual(t, "Timestamps should never be equal", times[j], times[k])
		}
	}

	waitSize := 3

	spin = spinner{}
	spin.Resolution = defaultSpinResolution

	times = make([]Timestamp, size)

	var wg sync.WaitGroup
	wg.Add(waitSize)
	mutex := &sync.Mutex{}

	var index int32

	for i := 0; i < waitSize; i++ {
		go func() {
			defer wg.Done()
			for {
				mutex.Lock()
				j := atomic.LoadInt32(&index)
				atomic.AddInt32(&index, 1)
				if j >= int32(size) {
					mutex.Unlock()
					break
				}
				times[j] = spin.next()
				mutex.Unlock()
			}
		}()
	}
	wg.Wait()

	for j := size - 1; j >= 0; j-- {
		for k := 0; k < size; k++ {
			if k == j {
				continue
			}
			assert.NotEqual(t, "Timestamps should never be equal", times[j], times[k])
		}
	}
}
