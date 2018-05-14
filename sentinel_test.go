package sentinel

import (
	"errors"
	"testing"

	"github.com/arwx/go-sentinel/mocks"

	"github.com/gomodule/redigo/redis"
	"github.com/stretchr/testify/assert"
)

func TestQueryMaster(t *testing.T) {
	snt := New(Config{
		Addrs:  []string{"localhost:26379"},
		Groups: []string{"example"},
	})

	conn := new(mocks.Conn)

	connResp := []interface{}{"172.16.0.1", "6379"}

	conn.Mock.On("Do", "SENTINEL", "get-master-addr-by-name", "example").Return(connResp, nil)

	master, err := snt.queryMaster(conn, "example")

	conn.Mock.AssertExpectations(t)

	assert.Nil(t, err)
	assert.Equal(t, "172.16.0.1:6379", master)
}

func TestQueryMaster_ConnNilReply(t *testing.T) {
	snt := New(Config{
		Addrs:  []string{"localhost:26379"},
		Groups: []string{"example"},
	})

	conn := new(mocks.Conn)

	conn.Mock.On("Do", "SENTINEL", "get-master-addr-by-name", "example").Return(nil, redis.ErrNil)

	master, err := snt.queryMaster(conn, "example")

	conn.Mock.AssertExpectations(t)

	assert.Equal(t, errMasterNameNotFound, err)
	assert.Empty(t, master)
}

func TestQueryMaster_ConnError(t *testing.T) {
	snt := New(Config{
		Addrs:  []string{"localhost:26379"},
		Groups: []string{"example"},
	})

	conn := new(mocks.Conn)

	connErr := errors.New("dial timeout")

	conn.Mock.On("Do", "SENTINEL", "get-master-addr-by-name", "example").Return(nil, connErr)

	master, err := snt.queryMaster(conn, "example")

	conn.Mock.AssertExpectations(t)

	assert.Equal(t, connErr, err)
	assert.Empty(t, master)
}

func TestGetMasterAddr(t *testing.T) {
	snt := New(Config{
		Addrs:  []string{"localhost:26379"},
		Groups: []string{"example"},
	})

	snt.groups["example"].syncMaster("172.16.0.1:6379")

	addr, err := snt.GetMasterAddr("example")

	assert.Nil(t, err)
	assert.Equal(t, "172.16.0.1:6379", addr)
}

func TestGetMasterAddr_NotDiscovered(t *testing.T) {
	snt := New(Config{
		Addrs:  []string{"localhost:26379"},
		Groups: []string{"example"},
	})

	addr, err := snt.GetMasterAddr("example")

	assert.Equal(t, ErrMasterUnavailable, err)
	assert.Empty(t, addr)
}

func TestGetMasterAddr_GroupNotRegistered(t *testing.T) {
	snt := New(Config{
		Addrs: []string{"localhost:26379"},
	})

	addr, err := snt.GetMasterAddr("example")

	assert.Equal(t, ErrInvalidMasterName, err)
	assert.Empty(t, addr)
}

func TestGetSlavesAddrs(t *testing.T) {
	snt := New(Config{
		Addrs:  []string{"localhost:26379"},
		Groups: []string{"example"},
	})

	slavesAddrs := []string{
		"172.16.0.1:6379",
		"172.16.0.1:6380",
	}

	snt.groups["example"].syncSlaves(slavesAddrs)

	addrs, err := snt.GetSlavesAddrs("example")

	assert.Nil(t, err)
	assert.Equal(t, slavesAddrs, addrs)
}

func TestGetSlavesAddrs_GroupNotRegistered(t *testing.T) {
	snt := New(Config{
		Addrs: []string{"localhost:26379"},
	})

	addrs, err := snt.GetSlavesAddrs("example")

	assert.Equal(t, ErrInvalidMasterName, err)
	assert.Nil(t, addrs)
}

func TestHandleNotification_SwitchMaster(t *testing.T) {
	snt := New(Config{
		Addrs:  []string{"localhost:26379"},
		Groups: []string{"example"},
	})

	snt.groups["example"].syncMaster("172.16.0.1:6379")

	snt.handleNotification(redis.Message{
		Channel: "+switch-master",
		Data:    []byte("example 172.16.0.1 6379 172.16.0.2 6380"),
	})

	master, err := snt.GetMasterAddr("example")

	assert.Nil(t, err)
	assert.Equal(t, "172.16.0.2:6380", master)
}

func TestHandleNotification_SlaveUp(t *testing.T) {
	snt := New(Config{
		Addrs:  []string{"localhost:26379"},
		Groups: []string{"example"},
	})

	// new slave 172.16.0.1 6380
	snt.handleNotification(redis.Message{
		Channel: "+slave",
		Data:    []byte("slave <name> 172.16.0.1 6380 @ example 172.16.0.1 6379"),
	})

	slaves, err := snt.GetSlavesAddrs("example")

	assert.Nil(t, err)
	assert.Equal(t, []string{"172.16.0.1:6380"}, slaves)

	// slave 172.16.0.1 is available again
	snt.handleNotification(redis.Message{
		Channel: "-sdown",
		Data:    []byte("slave <name> 172.16.0.1 6381 @ example 172.16.0.1 6379"),
	})

	slaves, err = snt.GetSlavesAddrs("example")

	assert.Nil(t, err)
	assert.Equal(t, []string{"172.16.0.1:6380", "172.16.0.1:6381"}, slaves)

	// new slave 172.16.0.1, should not duplicate
	snt.handleNotification(redis.Message{
		Channel: "+slave",
		Data:    []byte("slave <name> 172.16.0.1 6381 @ example 172.16.0.1 6379"),
	})

	slaves, err = snt.GetSlavesAddrs("example")

	assert.Nil(t, err)
	assert.Equal(t, []string{"172.16.0.1:6380", "172.16.0.1:6381"}, slaves)
}

func TestHandleNotification_SlaveDown(t *testing.T) {
	snt := New(Config{
		Addrs:  []string{"localhost:26379"},
		Groups: []string{"example"},
	})

	snt.groups["example"].syncSlaves([]string{"172.16.0.1:6380"})

	snt.handleNotification(redis.Message{
		Channel: "+sdown",
		Data:    []byte("slave <name> 172.16.0.1 6380 @ example 172.16.0.1 6379"),
	})

	slaves, err := snt.GetSlavesAddrs("example")

	assert.Nil(t, err)
	assert.Empty(t, slaves)
}

func TestSentinelAddrs(t *testing.T) {
	addrs := []string{
		"172.16.0.1:26379",
		"172.16.0.2:26379",
	}

	snt := &Sentinel{addrs: addrs}

	assert.Equal(t, addrs[0], snt.getSentinelAddr())

	snt.shiftSentinelAddr()

	assert.Equal(t, addrs[1], snt.getSentinelAddr())
}
