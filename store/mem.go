package store

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

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

type itemStack struct {
	ID    string   `json:"id"`
	Items [][]byte `json:"items"`
}

func (is *itemStack) StoreID() string {
	return is.ID
}

func (is *itemStack) StoreExpires() time.Time {
	return NeverExpire
}

func (s storage) Push(stack string, item Storable) error {
	keyName := fmt.Sprintf("_stack_%s", stack)
	var is itemStack
	var err error
	if err = s.FindByID(keyName, &is); err != nil && err != ErrItemNotFound {
		return err
	}

	if err == ErrItemNotFound {
		is.ID = keyName
		is.Items = [][]byte{}
	}

	data, err := json.Marshal(item)

	is.Items = append(is.Items, data)

	return s.Save(&is)
}

func (s storage) Pop(stack string, out interface{}) error {
	keyName := fmt.Sprintf("_stack_%s", stack)
	var is itemStack
	var err error
	var rawItem []byte
	if err = s.FindByID(keyName, &is); err != nil {
		return err
	}

	rawItem, is.Items = is.Items[0], is.Items[1:]

	err = s.Save(&is)
	if err != nil {
		return err
	}

	return json.Unmarshal(rawItem, out)
}

func (s storage) All(stack string, cb func(out []byte) error) error {
	keyName := fmt.Sprintf("_stack_%s", stack)
	var is itemStack
	var err error
	if err = s.FindByID(keyName, &is); err != nil {
		return err
	}

	for _, item := range is.Items {
		if err = cb(item); err != nil {
			return err
		}
	}

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
