package pubsub

import (
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// bufferSize defines the default size of the buffer for message subscribers.
// maxSubscribers specifies the maximum number of subscribers allowed.
const (
	bufferSize     = 64
	maxSubscribers = 128
)

// ErrAlreadyClosed indicates that the pubsub adapter is closed and cannot be used.
// ErrAlreadyOpen indicates that the pubsub adapter is already open.
// ErrMaxSub indicates that the pubsub adapter has reached its maximum number of subscribers.
// ErrDrops indicates that a message was dropped by some subscriber.
var (
	ErrAlreadyClosed     = errors.New("pubsub adapter is closed")
	ErrAlreadyOpen       = errors.New("pubsub adapter is open")
	ErrAlreadyExist      = errors.New("pubsub subscriber already exist")
	ErrMaxSub            = errors.New("pubsub adapter has reached max subscriber")
	ErrDrops             = errors.New("message dropped by some subscriber")
	ErrSubscriberMissing = errors.New("subscriber not found")
	ErrTopicMissing      = errors.New("topic not found")
)

// Adapter represents an in-memory pub-sub for managing subscribers and broadcasting messages concurrently.
type Adapter[T any] struct {
	mu             sync.RWMutex
	subs           map[string]chan T
	BufferSize     int
	MaxSubscribers int
	closed         bool
}

// NewAdapter initializes and returns a new instance of Adapter with default configurations and the empty subscriber map.
func NewAdapter[T any]() *Adapter[T] {
	ps := &Adapter[T]{}

	ps.subs = make(map[string]chan T)

	ps.MaxSubscribers = maxSubscribers

	ps.BufferSize = bufferSize

	return ps
}

// Publish sends the provided object to all active subscribers, skipping over any full channels without blocking.
// Returns an error if the adapter is closed or if any subscribers drop the message.
func (ps *Adapter[T]) Publish(obj T) error {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if ps.closed {
		return fmt.Errorf("not possible to publish to subscribers from a closed adapter: %w", ErrAlreadyClosed)
	}

	dropped := 0

	for _, ch := range ps.subs {
		// drop the message if chan buffer is full to avoid blocking state
		select {
		case ch <- obj:
		default:
			dropped++
		}
	}

	if dropped > 0 {
		return fmt.Errorf("%d susbscriber dropped the message :%w", dropped, ErrDrops)
	}

	return nil
}

// PublishExclude sends the provided object to all active subscribers exclude the added subscriber ID, skipping over any full channels without blocking.
// Returns an error if the adapter is closed or if any subscribers drop the message.
func (ps *Adapter[T]) PublishExclude(subID string, obj T) error {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if ps.closed {
		return fmt.Errorf("not possible to publish to subscribers from a closed adapter: %w", ErrAlreadyClosed)
	}

	dropped := 0

	for id, ch := range ps.subs {
		// skip the message if the subscriber is the one who send the message
		if subID == id {
			continue
		}

		// drop the message if chan buffer is full to avoid blocking state
		select {
		case ch <- obj:
		default:
			dropped++
		}
	}

	if dropped > 0 {
		return fmt.Errorf("%d susbscriber dropped the message :%w", dropped, ErrDrops)
	}

	return nil
}

// Subscribe registers an update callback to receive published messages and returns a unique subscription ID or an error.
func (ps *Adapter[T]) Subscribe(update func(obj T)) (string, error) {
	subID := uuid.New().String()

	return ps.SubscribeWithID(subID, update)
}

// SubscribeWithID registers a new subscriber with a specific ID and handler function and returns the subscriber ID or an error.
// It locks the Adapter instance, checks if it has reached its maximum subscribers, and spawns a goroutine for the handler.
func (ps *Adapter[T]) SubscribeWithID(subID string, handler func(obj T)) (string, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ch := make(chan T, ps.BufferSize)

	if ps.closed {
		return "", fmt.Errorf("not possible to subscribe from a closed adapter: %w", ErrAlreadyClosed)
	}

	if len(ps.subs) >= ps.MaxSubscribers {
		return "", fmt.Errorf("not possible to subscribe max number of subscriber reached: %w", ErrMaxSub)
	}

	ps.subs[subID] = ch

	go ps.runSubscriber(ch, handler)

	return subID, nil
}

// PublishTo sends an object to a specific subscriber channel using the subscriber's unique ID.
// Returns an error if the adapter is closed or the operation fails.
func (ps *Adapter[T]) PublishTo(subID string, obj T) error {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if ps.closed {
		return fmt.Errorf("not possible to publish to a subscriber from a closed adapter: %w", ErrAlreadyClosed)
	}

	ch, ok := ps.subs[subID]
	if !ok {
		return fmt.Errorf("cna't found subscriber id: %w", ErrSubscriberMissing)
	}

	select {
	case ch <- obj:
	default:
		return fmt.Errorf("buffer overflow from subscriber: %w", ErrDrops)
	}

	return nil
}

// PublishTo sends an object to a specific subscriber channel using the subscriber's unique ID.
// Returns an error if the adapter is closed or the operation fails.
func (ps *Adapter[T]) PublishToExclude(subID string, obj T) error {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if ps.closed {
		return fmt.Errorf("not possible to publish to a subscriber from a closed adapter: %w", ErrAlreadyClosed)
	}

	ch, ok := ps.subs[subID]
	if !ok {
		return fmt.Errorf("cna't found subscriber id: %w", ErrSubscriberMissing)
	}

	select {
	case ch <- obj:
	default:
		return fmt.Errorf("buffer overflow from subscriber: %w", ErrDrops)
	}

	return nil
}

// HasSubscriber checks if a subscriber with the given ID exists in the adapter, returning a boolean and an error.
func (ps *Adapter[T]) HasSubscriber(subID string) (bool, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if ps.closed {
		return false, fmt.Errorf("not possible to check subscriber from a closed adapter: %w", ErrAlreadyClosed)
	}

	_, exist := ps.subs[subID]

	return exist, nil
}

// AddSubscriber add a new subscriber to the adapter
func (ps *Adapter[T]) AddSubscriber(subID string, sub chan T) error {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if ps.closed {
		return fmt.Errorf("not possible to check subscriber from a closed adapter: %w", ErrAlreadyClosed)
	}

	_, exist := ps.subs[subID]
	if exist {
		return fmt.Errorf("can not add subscriber: %w", ErrAlreadyExist)
	}

	ps.subs[subID] = sub

	return nil
}

// GetSubscriber checks if a subscriber with the given ID exists in the adapter, returning a boolean and an error.
func (ps *Adapter[T]) GetSubscriber(subID string) (chan T, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if ps.closed {
		return nil, fmt.Errorf("not possible to check subscriber from a closed adapter: %w", ErrAlreadyClosed)
	}

	sub, exist := ps.subs[subID]
	if !exist {
		return nil, fmt.Errorf("subscriber not found: %w", ErrSubscriberMissing)
	}

	return sub, nil
}

// Subscribers retrieve a list of all active subscriber IDs. Returns an error if the adapter is closed.
func (ps *Adapter[T]) Subscribers() ([]string, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if ps.closed {
		return nil, fmt.Errorf("not possible to get subscriber from a closed adapter: %w", ErrAlreadyClosed)
	}

	subs := make([]string, 0, len(ps.subs))
	for k := range ps.subs {
		subs = append(subs, k)
	}

	return subs, nil
}

// Unsubscribe removes the subscriber identified by the given subID, closing its channel and preventing further updates.
func (ps *Adapter[T]) Unsubscribe(subID string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.closed {
		return fmt.Errorf("not possible to unsebscribe to a closed adapter: %w", ErrAlreadyClosed)
	}

	if ch, ok := ps.subs[subID]; ok {
		close(ch)

		delete(ps.subs, subID)
	}

	return nil
}

// UnsubscribeNoClose removes the subscriber identified by the given subID, don't closing its channel
func (ps *Adapter[T]) UnsubscribeNoClose(subID string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.closed {
		return fmt.Errorf("not possible to unsubscribe to a closed adapter: %w", ErrAlreadyClosed)
	}

	if _, ok := ps.subs[subID]; ok {
		delete(ps.subs, subID)
	}

	return nil
}

// Close safely closes the Adapter, releasing all resources and closing all active subscriber channels.
func (ps *Adapter[T]) Close() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.closed {
		return fmt.Errorf("adapter is already closed: %w", ErrAlreadyOpen)
	}

	ps.closed = true

	for subID, subs := range ps.subs {
		close(subs)

		delete(ps.subs, subID)
	}

	return nil
}

func (ps *Adapter[T]) Closed() bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	return ps.closed
}

// Open initializes the adapter by creating a new subscribers map and setting the adapter state to open (not closed).
func (ps *Adapter[T]) Open() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if !ps.closed {
		return fmt.Errorf("adapter is already open: %w", ErrAlreadyOpen)
	}

	ps.closed = false

	return nil
}

// runSubscriber processes messages received on the provided channel and invokes the handler function for each message.
func (ps *Adapter[T]) runSubscriber(ch <-chan T, handler func(obj T)) {
	for obj := range ch {
		// handle synchronously; backpressure via channel buffer
		handler(obj)
	}
}
