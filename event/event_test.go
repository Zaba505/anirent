package event

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBus(t *testing.T) {
	t.Run("subscribers should not receive events already processed", func(subT *testing.T) {
		doneCh := make(chan struct{}, 1)
		bus := NewBus[string]()

		go func() {
			for {
				select {
				case <-doneCh:
					return
				case <-time.After(1 * time.Millisecond):
					bus.Publish("test", "hello")
				}
			}
		}()

		<-time.After(500 * time.Millisecond)

		// approximately 50 events should have already been published by now
		var evs []string
		unsubscribe, err := bus.Subscribe("test", func(ev string) {
			evs = append(evs, ev)
		})
		if !assert.Nil(subT, err) {
			return
		}

		<-time.After(50 * time.Millisecond)
		unsubscribe()
    close(doneCh)

		// len(evs) == 50 +- 10
		assert.InDelta(subT, 50, len(evs), 10)
	})
}
