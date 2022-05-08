package event

import (
	"fmt"
	"sync"
)

// Bus implements a FIFO event bus.
type Bus[T any] struct {
	mu      sync.RWMutex
	streams map[string]*stream[T]
}

func NewBus[T any]() *Bus[T] {
	return &Bus[T]{
		streams: make(map[string]*stream[T]),
	}
}

func (b *Bus[T]) NewStream(id string) *stream[T] {
	s := &stream[T]{
		events:      make(chan T),
		done:        make(chan struct{}),
		subscribers: make(map[int]func(T)),
	}
	go s.start()

	b.mu.Lock()
	b.streams[id] = s
	b.mu.Unlock()
	return s
}

func (b *Bus[_]) closeStream(id string) {
	b.mu.Lock()
	s, exists := b.streams[id]
	if !exists {
		return
	}
	close(s.done)
	delete(b.streams, id)
	b.mu.Unlock()
}

func (b *Bus[T]) Publish(id string, ev T) {
	b.mu.RLock()
	s, exists := b.streams[id]
	b.mu.RUnlock()

	if !exists {
		s = b.NewStream(id)
	}

	go s.publish(ev)
}

func (b *Bus[T]) Subscribe(id string, f func(ev T)) (func(), error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	s, exists := b.streams[id]
	if !exists {
		return nil, fmt.Errorf("bus: no stream with id - %s", id)
	}

	return s.subscribe(f), nil
}

type stream[T any] struct {
	events chan T
	done   chan struct{}

	mu          sync.RWMutex
	n           int
	subscribers map[int]func(T)
}

func (s *stream[_]) start() {
	for {
		select {
		case <-s.done:
			return
		case event := <-s.events:
			s.mu.RLock()
			for _, subscriber := range s.subscribers {
				subscriber(event)
			}
			s.mu.RUnlock()
		}
	}
}

func (s *stream[T]) publish(ev T) {
	select {
	case <-s.done:
	case s.events <- ev:
	}
}

func (s *stream[T]) subscribe(f func(ev T)) func() {
	s.mu.Lock()
	n := s.n
	s.subscribers[n] = f
	s.n += 1
	s.mu.Unlock()

	return func() {
		s.mu.Lock()
		delete(s.subscribers, n)
		s.mu.Unlock()
	}
}
