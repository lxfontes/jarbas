package store

import (
	"errors"
	"time"
)

var (
	NeverExpire     = time.Time{}
	ErrItemNotFound = errors.New("not found")
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

	Push(stack string, item Storable) error
	Pop(stack string, out interface{}) error
	All(stack string, cb func(out []byte) error) error
}

type Store interface {
	Namespace(name string) Namespace
}
