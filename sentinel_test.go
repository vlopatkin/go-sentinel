package sentinel

import (
	"reflect"
	"testing"

	"github.com/garyburd/redigo/redis"
)

func TestGetMasterAddr(t *testing.T) {
	masterName := "master1"
	masterAddr := "172.16.0.1:6379"

	snt := &Sentinel{
		groups: map[string]*group{
			masterName: &group{
				master: masterAddr,
			},
		},
	}

	addr, err := snt.GetMasterAddr(masterName)

	if err != nil {
		t.Errorf("expected nil error\ngot: %v", err)
	}

	if addr != masterAddr {
		t.Errorf("expected addr: %s\ngot: %s", masterAddr, addr)
	}
}

func TestGetMasterAddr_NotDiscovered(t *testing.T) {
	masterName := "master1"

	snt := &Sentinel{
		groups: map[string]*group{
			masterName: &group{},
		},
	}

	addr, err := snt.GetMasterAddr(masterName)

	if err != ErrMasterUnavailable {
		t.Errorf("expected error: %v\ngot: %v", ErrMasterUnavailable, err)
	}

	if addr != "" {
		t.Errorf("expected empty addr\ngot: %s", addr)
	}
}

func TestGetMasterAddr_NotRegistered(t *testing.T) {
	snt := &Sentinel{}

	addr, err := snt.GetMasterAddr("master1")

	if err != ErrInvalidMasterName {
		t.Errorf("expected error: %v\ngot: %v", ErrInvalidMasterName, err)
	}

	if addr != "" {
		t.Errorf("expected empty addr\ngot: %s", addr)
	}
}

func TestGetSlavesAddrs(t *testing.T) {
	masterName := "master1"
	slavesAddrs := []string{
		"172.16.0.1:6379", "172.16.0.1:6380",
	}

	snt := &Sentinel{
		groups: map[string]*group{
			masterName: &group{
				slaves: slavesAddrs,
			},
		},
	}

	addrs, err := snt.GetSlavesAddrs(masterName)

	if err != nil {
		t.Errorf("expected nil error\ngot: %v", err)
	}

	if !reflect.DeepEqual(addrs, slavesAddrs) {
		t.Errorf("expected addrs: %s\ngot: %s", slavesAddrs, addrs)
	}
}

func TestGetSlavesAddrs_NotRegistered(t *testing.T) {
	snt := &Sentinel{}

	addrs, err := snt.GetSlavesAddrs("master1")

	if err != ErrInvalidMasterName {
		t.Errorf("expected error: %v\ngot: %v", ErrInvalidMasterName, err)
	}

	if addrs != nil {
		t.Errorf("expected empty addrs\ngot: %v", addrs)
	}
}

func TestHandleNotification_SwitchMaster(t *testing.T) {
	masterName := "master1"

	snt := &Sentinel{
		groups: map[string]*group{
			masterName: &group{
				master: "172.16.0.1:6379",
			},
		},
	}

	exp := "172.16.0.2:6380"

	snt.handleNotification(redis.Message{
		Channel: "+switch-master",
		Data:    []byte("master1 172.16.0.1 6379 172.16.0.2 6380"),
	})

	addr := snt.groups[masterName].master

	if addr != exp {
		t.Errorf("expected addr: %s\ngot: %s", exp, addr)
	}
}

func Test_Addrs(t *testing.T) {
	addrs := []string{"172.16.0.1:26379", "172.16.0.2:26379"}

	snt := &Sentinel{addrs: addrs}

	addr := snt.getAddr()

	if addr != addrs[0] {
		t.Errorf("expected addr: %s\ngot: %s", addrs[0], addr)
	}

	snt.shiftAddr(addr)

	addr = snt.getAddr()

	if addr != addrs[1] {
		t.Errorf("expected addr: %s\ngot: %s", addrs[1], addr)
	}
}

func TestNew(t *testing.T) {
	addrs := []string{"172.16.0.1:26379"}
	groups := []string{"master1", "master2"}

	snt := New(&Config{
		Addrs:  addrs,
		Groups: groups,
	})

	if !reflect.DeepEqual(snt.addrs, addrs) {
		t.Errorf("expected addrs: %v\ngot: %v", addrs, snt.addrs)
	}
}
