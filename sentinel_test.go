package sentinel

import (
	"errors"
	"testing"

	"github.com/ncade/go-sentinel/mocks"

	"github.com/gomodule/redigo/redis"
	"github.com/stretchr/testify/assert"
)

func TestSentinelQueryMaster(t *testing.T) {
	conn := new(mocks.Conn)

	resp := []interface{}{"172.16.0.1", "6379"}

	conn.Mock.On("Do", "SENTINEL", "get-master-addr-by-name", "redis01").Return(resp, nil)

	master, err := (&Sentinel{}).queryMaster(conn, "redis01")

	conn.Mock.AssertExpectations(t)

	assert.Nil(t, err)
	assert.Equal(t, "172.16.0.1:6379", master)
}

func TestSentinelQueryMaster_ConnNilReply(t *testing.T) {
	conn := new(mocks.Conn)

	conn.Mock.On("Do", "SENTINEL", "get-master-addr-by-name", "redis01").Return(nil, redis.ErrNil)

	master, err := (&Sentinel{}).queryMaster(conn, "redis01")

	conn.Mock.AssertExpectations(t)

	assert.Equal(t, errMasterNameNotFound, err)
	assert.Empty(t, master)
}

func TestSentinelQueryMaster_ConnError(t *testing.T) {
	conn := new(mocks.Conn)

	connErr := errors.New("dial timeout")

	conn.Mock.On("Do", "SENTINEL", "get-master-addr-by-name", "redis01").Return(nil, connErr)

	master, err := (&Sentinel{}).queryMaster(conn, "redis01")

	conn.Mock.AssertExpectations(t)

	assert.Equal(t, connErr, err)
	assert.Empty(t, master)
}

func TestSentinelGetMasterAddr(t *testing.T) {
	snt := &Sentinel{
		groups: map[string]*group{
			"redis01": {
				master: "172.16.0.1:6379",
			},
		},
	}

	addr, err := snt.GetMasterAddr("redis01")

	assert.Nil(t, err)
	assert.Equal(t, "172.16.0.1:6379", addr)
}

func TestSentinelGetMasterAddr_NotDiscovered(t *testing.T) {
	snt := &Sentinel{
		groups: map[string]*group{
			"redis01": {},
		},
	}

	addr, err := snt.GetMasterAddr("redis01")

	assert.Equal(t, ErrMasterUnavailable, err)
	assert.Empty(t, addr)
}

func TestSentinelGetMasterAddr_GroupNotRegistered(t *testing.T) {
	addr, err := (&Sentinel{}).GetMasterAddr("redis01")

	assert.Equal(t, ErrInvalidMasterName, err)
	assert.Empty(t, addr)
}

func TestSentinelGetSlavesAddrs(t *testing.T) {
	slavesAddrs := []string{
		"172.16.0.1:6379",
		"172.16.0.1:6380",
	}

	snt := &Sentinel{
		groups: map[string]*group{
			"redis01": {
				slaves: slavesAddrs,
			},
		},
	}

	addrs, err := snt.GetSlavesAddrs("redis01")

	assert.Nil(t, err)
	assert.Equal(t, slavesAddrs, addrs)
}

func TestSentinelGetSlavesAddrs_GroupNotRegistered(t *testing.T) {
	addrs, err := (&Sentinel{}).GetSlavesAddrs("redis01")

	assert.Equal(t, ErrInvalidMasterName, err)
	assert.Nil(t, addrs)
}

func TestSentinelHandleNotification_SwitchMaster(t *testing.T) {
	snt := &Sentinel{
		groups: map[string]*group{
			"redis01": {master: "172.16.0.1:6379"},
		},
	}

	snt.handleNotification(redis.Message{
		Channel: "+switch-master",
		Data:    []byte("redis01 172.16.0.1 6379 172.16.0.2 6380"),
	})

	master, err := snt.GetMasterAddr("redis01")

	assert.Nil(t, err)
	assert.Equal(t, "172.16.0.2:6380", master)
}

func TestSentinelHandleNotification_SlaveUp(t *testing.T) {
	snt := &Sentinel{
		groups: map[string]*group{
			"redis01": {},
		},
	}

	// new slave 172.16.0.1:6380
	snt.handleNotification(redis.Message{
		Channel: "+slave",
		Data:    []byte("slave <name> 172.16.0.1 6380 @ redis01 172.16.0.1 6379"),
	})

	slaves, err := snt.GetSlavesAddrs("redis01")

	assert.Nil(t, err)
	assert.Equal(t, []string{"172.16.0.1:6380"}, slaves)

	// slave 172.16.0.1:6381 is available again
	snt.handleNotification(redis.Message{
		Channel: "-sdown",
		Data:    []byte("slave <name> 172.16.0.1 6381 @ redis01 172.16.0.1 6379"),
	})

	slaves, err = snt.GetSlavesAddrs("redis01")

	assert.Nil(t, err)
	assert.Equal(t, []string{"172.16.0.1:6380", "172.16.0.1:6381"}, slaves)

	// new slave 172.16.0.1:6381 should not duplicate
	snt.handleNotification(redis.Message{
		Channel: "+slave",
		Data:    []byte("slave <name> 172.16.0.1 6381 @ redis01 172.16.0.1 6379"),
	})

	slaves, err = snt.GetSlavesAddrs("redis01")

	assert.Nil(t, err)
	assert.Equal(t, []string{"172.16.0.1:6380", "172.16.0.1:6381"}, slaves)
}

func TestSentinelHandleNotification_SlaveDown(t *testing.T) {
	snt := &Sentinel{
		groups: map[string]*group{
			"redis01": {slaves: []string{"172.16.0.1:6380"}},
		},
	}

	snt.handleNotification(redis.Message{
		Channel: "+sdown",
		Data:    []byte("slave <name> 172.16.0.1 6380 @ redis01 172.16.0.1 6379"),
	})

	slaves, err := snt.GetSlavesAddrs("redis01")

	assert.Nil(t, err)
	assert.Empty(t, slaves)
}

func TestSentinelTestRole(t *testing.T) {
	conn := new(mocks.Conn)

	snt := &Sentinel{
		connDial: func(addr string) (redis.Conn, error) {
			return conn, nil
		},
	}

	conn.Mock.On("Do", "ROLE").Return([]interface{}{"master", 3129659}, nil)

	err := snt.testRole("172.16.0.1:6379", "master")

	conn.Mock.AssertExpectations(t)

	assert.Nil(t, err)
}

func TestSentinelTestRole_DifferentRole(t *testing.T) {
	conn := new(mocks.Conn)

	snt := &Sentinel{
		connDial: func(addr string) (redis.Conn, error) {
			return conn, nil
		},
	}

	conn.Mock.On("Do", "ROLE").Return([]interface{}{"slave", "172.16.0.1", 6379}, nil)

	err := snt.testRole("172.16.0.1:6379", "master")

	conn.Mock.AssertExpectations(t)

	assert.NotNil(t, err)
	assert.Equal(t, "172.16.0.1:6379 invalid role: slave", err.Error())
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

func TestSentinelNew(t *testing.T) {
	sentinelAddrs := []string{"172.16.0.1:26379", "172.16.0.2:26379"}

	snt := New(Config{
		Addrs:  sentinelAddrs,
		Groups: []string{"redis01", "redis02", "redis03"},
	})

	assert.Equal(t, sentinelAddrs, snt.addrs)
	assert.NotNil(t, snt.groups)
	assert.Equal(t, defaultRefreshInterval, snt.refreshInterval)
	assert.Equal(t, defaultHeartbeatInterval, snt.heartbeatInterval)
	assert.NotNil(t, snt.connDial)
	assert.NotNil(t, snt.psConnDial)
	assert.NotNil(t, snt.onError)
}
