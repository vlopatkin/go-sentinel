package sentinel

import (
	"reflect"
	"testing"

	"github.com/garyburd/redigo/redis"
)

func TestMasterAddr(t *testing.T) {
	masterAddr := "172.16.0.1:6379"

	snt := &Sentinel{
		groups: map[string]*group{
			"master1": &group{
				master: masterAddr,
			},
		},
	}

	addr, err := snt.MasterAddr("master1")

	if err != nil {
		t.Errorf("expected nil error\ngot: %v", err)
	}

	if addr != masterAddr {
		t.Errorf("expected addr: %v\ngot: %v", masterAddr, addr)
	}
}

func TestMasterAddr_NotDiscovered(t *testing.T) {
	snt := &Sentinel{
		groups: map[string]*group{
			"master1": &group{},
		},
	}

	addr, err := snt.MasterAddr("master1")

	if err != ErrMasterUnavailable {
		t.Errorf("expected error: %v\ngot: %v", ErrMasterUnavailable, err)
	}

	if addr != "" {
		t.Errorf("expected empty addr\ngot: %v", addr)
	}
}

func TestMasterAddr_NotRegistered(t *testing.T) {
	snt := &Sentinel{}

	addr, err := snt.MasterAddr("master1")

	if err != ErrInvalidMasterName {
		t.Errorf("expected error: %v\ngot: %v", ErrInvalidMasterName, err)
	}

	if addr != "" {
		t.Errorf("expected empty addr\ngot: %v", addr)
	}
}

func TestSlaveAddrs(t *testing.T) {
	slavesAddrs := []string{
		"172.16.0.1:6379", "172.16.0.1:6380",
	}

	snt := &Sentinel{
		groups: map[string]*group{
			"master1": &group{
				slaves: slavesAddrs,
			},
		},
	}

	addrs, err := snt.SlaveAddrs("master1")

	if err != nil {
		t.Errorf("expected nil error\ngot: %v", err)
	}

	if !reflect.DeepEqual(addrs, slavesAddrs) {
		t.Errorf("expected addrs: %v\ngot: %v", slavesAddrs, addrs)
	}
}

func TestSlaveAddrs_NotRegistered(t *testing.T) {
	snt := &Sentinel{}

	addrs, err := snt.SlaveAddrs("master1")

	if err != ErrInvalidMasterName {
		t.Errorf("expected error: %v\ngot: %v", ErrInvalidMasterName, err)
	}

	if addrs != nil {
		t.Errorf("expected empty addrs\ngot: %v", addrs)
	}
}

func TestHandleNotification_SwitchMaster(t *testing.T) {
	snt := &Sentinel{
		groups: map[string]*group{
			"master1": &group{
				master: "172.16.0.1:6379",
			},
		},
	}

	exp := "172.16.0.2:6380"

	snt.handleNotification(redis.Message{
		Channel: "+switch-master",
		Data:    []byte("master1 172.16.0.1 6379 172.16.0.2 6380"),
	})

	addr := snt.groups["master1"].master

	if addr != exp {
		t.Errorf("expected addr: %s\ngot: %s", exp, addr)
	}
}

func TestNew(t *testing.T) {
	hosts := []string{"172.16.0.1:26379"}
	groups := []string{"master1", "master2"}

	snt := New(&Config{
		Hosts:  hosts,
		Groups: groups,
	})

	if !reflect.DeepEqual(snt.hosts, hosts) {
		t.Errorf("expected hosts: %v\ngot: %v", hosts, snt.hosts)
	}
}
