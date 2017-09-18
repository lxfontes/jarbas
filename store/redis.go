package store

import (
	"encoding/json"
	"fmt"

	"github.com/garyburd/redigo/redis"
)

const (
	// max idle connections in the pool
	redisMaxIdle = 5
)

type redisStore struct {
	pool *redis.Pool
}

type redisNamespace struct {
	redisStore *redisStore
	namespace  string
}

func (rn *redisNamespace) keyFor(k string) string {
	return fmt.Sprintf("%s:%s", rn.namespace, k)
}

func (rn *redisNamespace) FindByID(id string, out interface{}) error {
	client := rn.redisStore.conn()
	defer client.Close()

	resp, err := client.Do("GET", rn.keyFor(id))
	if err != nil {
		return err
	}

	rawItem, err := redis.Bytes(resp, err)
	if err != nil {
		if err == redis.ErrNil {
			return ErrItemNotFound
		}
		return err
	}

	return json.Unmarshal(rawItem, out)
}

func (rn *redisNamespace) Save(item Storable) error {
	client := rn.redisStore.conn()
	defer client.Close()

	rawItem, err := json.Marshal(item)
	if err != nil {
		return err
	}

	_, err = client.Do("SET", rn.keyFor(item.StoreID()), rawItem)

	return err
}

func (rn *redisNamespace) Delete(id string) error {
	client := rn.redisStore.conn()
	defer client.Close()

	_, err := client.Do("DEL", rn.keyFor(id))

	return err
}

func (rn *redisNamespace) Push(stack string, item Storable) error {
	client := rn.redisStore.conn()
	defer client.Close()

	rawItem, err := json.Marshal(item)
	if err != nil {
		return err
	}

	_, err = client.Do("RPUSH", rn.keyFor(stack), rawItem)

	return err
}

func (rn *redisNamespace) Pop(stack string, out interface{}) error {
	client := rn.redisStore.conn()
	defer client.Close()

	resp, err := client.Do("LPOP", rn.keyFor(stack))
	if err != nil {
		return err
	}

	rawItem, err := redis.Bytes(resp, err)
	if err != nil {
		if err == redis.ErrNil {
			return ErrItemNotFound
		}
		return err
	}

	return json.Unmarshal(rawItem, out)
}

func (rn *redisNamespace) All(stack string, cb func(out []byte) error) error {
	return nil
}

var _ Store = &redisStore{}
var _ Namespace = &redisNamespace{}

// TODO: hostname selector
func NewRedisStore() (*redisStore, error) {
	addr := "localhost:6379"
	dialer := func() (redis.Conn, error) { return redis.Dial("tcp", addr) }

	return &redisStore{
		pool: redis.NewPool(dialer, redisMaxIdle),
	}, nil
}

func (rs *redisStore) Namespace(name string) Namespace {
	return &redisNamespace{
		redisStore: rs,
		namespace:  name,
	}
}

func (rs *redisStore) conn() redis.Conn {
	return rs.pool.Get()
}
