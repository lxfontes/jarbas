package store

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
)

type stubItem struct {
	ID         string `json:"id"`
	Thing      string `json:"thing"`
	SomeNumber int    `json:"some_number"`
	expires    time.Time
}

var _ Storable = &stubItem{}

func (si *stubItem) StoreID() string {
	return si.ID
}

func (si *stubItem) StoreExpires() time.Time {
	return si.expires
}

func performStoreTest(t *testing.T, s Store) {
	t.Run("saveLoad", func(t *testing.T) {
		namespace := s.Namespace(uuid.New())

		id := "123"
		thing := "abc"
		someNumber := 666

		si := &stubItem{
			ID:         id,
			Thing:      thing,
			SomeNumber: someNumber,
		}

		if err := namespace.Save(si); err != nil {
			t.Fatal(err)
		}

		var stored stubItem

		if err := namespace.FindByID(id, &stored); err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, id, stored.ID)
		assert.Equal(t, thing, stored.Thing)
		assert.Equal(t, someNumber, stored.SomeNumber)
	})

	t.Run("saveDelete", func(t *testing.T) {
		namespace := s.Namespace(uuid.New())

		id := "123"
		si := &stubItem{
			ID: id,
		}

		if err := namespace.Save(si); err != nil {
			t.Fatal(err)
		}

		if err := namespace.Delete(id); err != nil {
			t.Fatal(err)
		}

		garbage := json.RawMessage{}
		err := namespace.FindByID(id, garbage)
		assert.Equal(t, ErrItemNotFound, err)
	})

	t.Run("inexistent key", func(t *testing.T) {
		namespace := s.Namespace(uuid.New())

		id := "lelulz"
		garbage := json.RawMessage{}
		err := namespace.FindByID(id, garbage)
		assert.Equal(t, ErrItemNotFound, err)

		err = namespace.Delete(id)
		assert.Nil(t, err)
	})

	t.Run("push", func(t *testing.T) {
		namespace := s.Namespace(uuid.New())

		stack := uuid.New()
		id := "123"
		si := &stubItem{
			ID: id,
		}

		err := namespace.Push(stack, si)
		assert.Nil(t, err)
	})

	t.Run("pop", func(t *testing.T) {
		namespace := s.Namespace(uuid.New())

		stack := uuid.New()
		id := "123"
		si := &stubItem{
			ID: id,
		}

		err := namespace.Push(stack, si)
		assert.Nil(t, err)

		var stored stubItem
		err = namespace.Pop(stack, &stored)
		assert.Nil(t, err)
		assert.Equal(t, id, stored.ID)
	})

	t.Run("all", func(t *testing.T) {
		namespace := s.Namespace(uuid.New())

		stack := uuid.New()
		id := "123"
		si := &stubItem{
			ID: id,
		}

		err := namespace.Push(stack, si)
		assert.Nil(t, err)
	})
}
