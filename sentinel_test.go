package sentinel

import (
	"testing"

	"github.com/garyburd/redigo/redis"
)

func TestHandleNotification_SwitchMaster(t *testing.T) {
	snt := New(&Config{
		Hosts:  []string{"172.16.0.1:26379"},
		Groups: []string{"master1"},
	})

	expAddr := "172.16.0.2:6380"

	snt.handleNotification(redis.Message{
		Channel: "+switch-master",
		Data:    []byte("master1 172.16.0.1 6379 172.16.0.2 6380"),
	})

	addr, _ := snt.MasterAddr("master1")

	if addr != expAddr {
		t.Errorf("expected addr: %s\ngot: %s", expAddr, addr)
	}

	expAddr = "172.16.0.1:6379"

	snt.handleNotification(redis.Message{
		Channel: "+switch-master",
		Data:    []byte("master1 172.16.0.2 6380 172.16.0.1 6379"),
	})

	addr, _ = snt.MasterAddr("master1")

	if addr != expAddr {
		t.Errorf("expected addr: %s\ngot: %s", expAddr, addr)
	}
}
