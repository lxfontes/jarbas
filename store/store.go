package store

import (
	"encoding/json"
	"errors"
	"sync"
)

type Storable interface {
	StoreID() string
}

type Store interface {
	FindByID(collection string, id string, out interface{}) error
	Save(collection string, item Storable) error
}

var _ Store = &memStore{}

type storage map[string][]byte

func (s storage) findByID(id string, out interface{}) error {
	rawItem, ok := s[id]
	if !ok {
		return errors.New("not found")
	}

	return json.Unmarshal(rawItem, out)
}

func (s storage) save(item Storable) error {
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

func (ms *memStore) FindByID(collection string, id string, out interface{}) error {
	ms.mtx.Lock()
	defer ms.mtx.Unlock()

	return ms.things[collection].findByID(id, out)
}

func (ms *memStore) Save(collection string, item Storable) error {
	ms.mtx.Lock()
	defer ms.mtx.Unlock()

	if _, ok := ms.things[collection]; !ok {
		ms.things[collection] = storage{}
	}

	return ms.things[collection].save(item)
}
