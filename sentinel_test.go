package sentinel

import (
	"testing"

	"github.com/garyburd/redigo/redis"
	"github.com/stretchr/testify/assert"
)

func TestGetMasterAddr(t *testing.T) {
	masterName := "example"
	masterAddr := "172.16.0.1:6379"

	snt := &Sentinel{
		groups: map[string]*group{
			masterName: {
				master: masterAddr,
			},
		},
	}

	addr, err := snt.GetMasterAddr(masterName)

	assert.Nil(t, err)
	assert.Equal(t, masterAddr, addr)
}

func TestGetMasterAddr_NotDiscovered(t *testing.T) {
	masterName := "example"

	snt := &Sentinel{
		groups: map[string]*group{
			masterName: {
				master: "",
			},
		},
	}

	addr, err := snt.GetMasterAddr(masterName)

	assert.Equal(t, ErrMasterUnavailable, err)
	assert.Empty(t, addr)
}

func TestGetMasterAddr_NotRegistered(t *testing.T) {
	snt := &Sentinel{
		groups: map[string]*group{},
	}

	addr, err := snt.GetMasterAddr("example")

	assert.Equal(t, ErrInvalidMasterName, err)
	assert.Empty(t, addr)
}

func TestGetSlavesAddrs(t *testing.T) {
	masterName := "example"

	slavesAddrs := []string{
		"172.16.0.1:6379",
		"172.16.0.1:6380",
	}

	snt := &Sentinel{
		groups: map[string]*group{
			masterName: {
				slaves: slavesAddrs,
			},
		},
	}

	addrs, err := snt.GetSlavesAddrs(masterName)

	assert.Nil(t, err)
	assert.Equal(t, slavesAddrs, addrs)
}

func TestGetSlavesAddrs_NotRegistered(t *testing.T) {
	snt := &Sentinel{
		groups: map[string]*group{},
	}

	addrs, err := snt.GetSlavesAddrs("example")

	assert.Equal(t, ErrInvalidMasterName, err)
	assert.Nil(t, addrs)
}

func TestHandleNotification_SwitchMaster(t *testing.T) {
	masterName := "example"

	snt := &Sentinel{
		groups: map[string]*group{
			masterName: {
				master: "172.16.0.1:6379",
			},
		},
	}

	snt.handleNotification(redis.Message{
		Channel: "+switch-master",
		Data:    []byte("example 172.16.0.1 6379 172.16.0.2 6380"),
	})

	master, err := snt.GetMasterAddr(masterName)

	assert.Nil(t, err)
	assert.Equal(t, "172.16.0.2:6380", master)
}

func TestHandleNotification_SlaveUp(t *testing.T) {
	masterName := "example"

	snt := &Sentinel{
		groups: map[string]*group{
			masterName: {
				slaves: []string{},
			},
		},
	}

	// new slave 172.16.0.1 6380
	snt.handleNotification(redis.Message{
		Channel: "+slave",
		Data:    []byte("slave <name> 172.16.0.1 6380 @ example 172.16.0.1 6379"),
	})

	slaves, err := snt.GetSlavesAddrs(masterName)

	assert.Nil(t, err)
	assert.Equal(t, []string{"172.16.0.1:6380"}, slaves)

	// slave 172.16.0.1 is available again
	snt.handleNotification(redis.Message{
		Channel: "-sdown",
		Data:    []byte("slave <name> 172.16.0.1 6381 @ example 172.16.0.1 6379"),
	})

	slaves, err = snt.GetSlavesAddrs(masterName)

	assert.Nil(t, err)
	assert.Equal(t, []string{"172.16.0.1:6380", "172.16.0.1:6381"}, slaves)

	// new slave 172.16.0.1, should not duplicate
	snt.handleNotification(redis.Message{
		Channel: "+slave",
		Data:    []byte("slave <name> 172.16.0.1 6381 @ example 172.16.0.1 6379"),
	})

	slaves, err = snt.GetSlavesAddrs(masterName)

	assert.Nil(t, err)
	assert.Equal(t, []string{"172.16.0.1:6380", "172.16.0.1:6381"}, slaves)
}

func TestHandleNotification_SlaveDown(t *testing.T) {
	masterName := "example"

	snt := &Sentinel{
		groups: map[string]*group{
			masterName: {
				slaves: []string{"172.16.0.1:6380"},
			},
		},
	}

	snt.handleNotification(redis.Message{
		Channel: "+sdown",
		Data:    []byte("slave <name> 172.16.0.1 6380 @ example 172.16.0.1 6379"),
	})

	slaves, err := snt.GetSlavesAddrs(masterName)

	assert.Nil(t, err)
	assert.Empty(t, slaves)
}

func Test_Addrs(t *testing.T) {
	addrs := []string{
		"172.16.0.1:26379",
		"172.16.0.2:26379",
	}

	snt := &Sentinel{addrs: addrs}

	assert.Equal(t, addrs[0], snt.getSentinelAddr())

	snt.shiftSentinelAddr()

	assert.Equal(t, addrs[1], snt.getSentinelAddr())
}

func TestNew(t *testing.T) {
	addrs := []string{"172.16.0.1:26379"}
	groups := []string{"example", "example1"}

	snt := New(Config{
		Addrs:  addrs,
		Groups: groups,
	})

	assert.Equal(t, addrs, snt.addrs)
	assert.NotNil(t, snt.groups[groups[0]])
	assert.NotNil(t, snt.groups[groups[1]])
}
