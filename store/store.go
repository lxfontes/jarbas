package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

type Storable interface {
	StoreID() string
}

type Store interface {
	FindByID(collection string, id string, out interface{}) error
	Save(collection string, item Storable) error
	Delete(collection string, id string) error
}

var ErrItemNotFound = errors.New("not found")

var _ Store = &memStore{}

type storage map[string][]byte

func (s storage) findByID(id string, out interface{}) error {
	rawItem, ok := s[id]
	if !ok {
		return ErrItemNotFound
	}

	return json.Unmarshal(rawItem, out)
}

func (s storage) delete(id string) error {
	delete(s, id)
	return nil
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

	fmt.Println("Looking up", collection, id)

	if _, ok := ms.things[collection]; !ok {
		ms.things[collection] = storage{}
	}

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

func (ms *memStore) Delete(collection string, id string) error {
	ms.mtx.Lock()
	defer ms.mtx.Unlock()

	col, ok := ms.things[collection]
	if !ok {
		return nil // no collection
	}

	return col.delete(id)
}
