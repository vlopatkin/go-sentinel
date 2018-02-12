package sentinel

import (
	"testing"

	"github.com/garyburd/redigo/redis"
	"github.com/stretchr/testify/assert"
)

func TestGetMasterAddr(t *testing.T) {
	masterName := "master1"
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
	masterName := "master1"

	snt := &Sentinel{
		groups: map[string]*group{
			masterName: {},
		},
	}

	addr, err := snt.GetMasterAddr(masterName)

	assert.Equal(t, ErrMasterUnavailable, err)
	assert.Empty(t, addr)
}

func TestGetMasterAddr_NotRegistered(t *testing.T) {
	snt := &Sentinel{}

	addr, err := snt.GetMasterAddr("master1")

	assert.Equal(t, ErrInvalidMasterName, err)
	assert.Empty(t, addr)
}

func TestGetSlavesAddrs(t *testing.T) {
	masterName := "master1"
	slavesAddrs := []string{
		"172.16.0.1:6379", "172.16.0.1:6380",
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
	snt := &Sentinel{}

	addrs, err := snt.GetSlavesAddrs("master1")

	assert.Equal(t, ErrInvalidMasterName, err)
	assert.Nil(t, addrs)
}

func TestHandleNotification_SwitchMaster(t *testing.T) {
	masterName := "master1"

	snt := &Sentinel{
		groups: map[string]*group{
			masterName: {
				master: "172.16.0.1:6379",
			},
		},
	}

	snt.handleNotification(redis.Message{
		Channel: "+switch-master",
		Data:    []byte("master1 172.16.0.1 6379 172.16.0.2 6380"),
	})

	master, err := snt.GetMasterAddr(masterName)

	assert.Nil(t, err)
	assert.Equal(t, "172.16.0.2:6380", master)
}

func TestHandleNotification_SlaveUp(t *testing.T) {
	masterName := "master1"

	snt := &Sentinel{
		groups: map[string]*group{
			masterName: {
				slaves: []string{},
			},
		},
	}

	snt.handleNotification(redis.Message{
		Channel: "+slave",
		Data:    []byte("slave <name> 172.16.0.1 6380 @ master1 172.16.0.1 6379"),
	})

	slaves, err := snt.GetSlavesAddrs(masterName)

	assert.Nil(t, err)
	assert.Equal(t, []string{"172.16.0.1:6380"}, slaves)

	snt.handleNotification(redis.Message{
		Channel: "-sdown",
		Data:    []byte("slave <name> 172.16.0.1 6381 @ master1 172.16.0.1 6379"),
	})

	slaves, err = snt.GetSlavesAddrs(masterName)

	assert.Nil(t, err)
	assert.Equal(t, []string{"172.16.0.1:6380", "172.16.0.1:6381"}, slaves)
}

func Test_Addrs(t *testing.T) {
	addrs := []string{
		"172.16.0.1:26379",
		"172.16.0.2:26379",
	}

	snt := &Sentinel{addrs: addrs}

	assert.Equal(t, addrs[0], snt.getAddr())

	snt.shiftAddr()

	assert.Equal(t, addrs[1], snt.getAddr())
}

func TestNew(t *testing.T) {
	addrs := []string{
		"172.16.0.1:26379",
	}

	groups := []string{
		"master1",
		"master2",
	}

	snt := New(&Config{
		Addrs:  addrs,
		Groups: groups,
	})

	sntGroups := make([]string, 0)

	for name := range snt.groups {
		sntGroups = append(sntGroups, name)
	}

	assert.Equal(t, addrs, snt.addrs)
	assert.Equal(t, groups, sntGroups)
}
