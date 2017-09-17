package store

import (
	"encoding/json"
	"errors"
	"sync"
	"time"
)

type Storable interface {
	StoreID() string
	// StoreExpires is a hint for stores on how durable this information is
	// leave blank for no expiration
	StoreExpires() time.Time
}

type Namespace interface {
	FindByID(id string, out interface{}) error
	Save(item Storable) error
	Delete(id string) error
}

type Store interface {
	Namespace(name string) Namespace
}

var ErrItemNotFound = errors.New("not found")

var _ Store = &memStore{}
var _ Namespace = storage{}

type storage map[string][]byte

func (s storage) FindByID(id string, out interface{}) error {
	rawItem, ok := s[id]
	if !ok {
		return ErrItemNotFound
	}

	return json.Unmarshal(rawItem, out)
}

func (s storage) Delete(id string) error {
	delete(s, id)
	return nil
}

func (s storage) Save(item Storable) error {
	rw, err := json.Marshal(item)
	if err != nil {
		return err
	}

	s[item.StoreID()] = rw
	return nil
}

type memStore struct {
	things map[string]storage
	mtx    sync.Mutex
}

func NewMemoryStore() *memStore {
	return &memStore{
		things: map[string]storage{},
	}
}

func (ms *memStore) Namespace(name string) Namespace {
	namespace, ok := ms.things[name]
	if !ok {
		namespace = storage{}
		ms.things[name] = namespace
	}

	return namespace
}
