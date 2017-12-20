package sentinel

import (
	"sync"
)

type group struct {
	name string

	mu     sync.RWMutex
	master string
	slaves []string
}

func (g *group) masterAddr() string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.master
}

func (g *group) syncMaster(addr string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.master = addr
}

func (g *group) slavesAddrs() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.slaves
}

func (g *group) syncSlaves(addrs []string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.slaves = addrs
}

func (g *group) syncSlaveUp(addr string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for _, saddr := range g.slaves {
		if saddr == addr {
			return
		}
	}

	g.slaves = append(g.slaves, addr)
}

func (g *group) syncSlaveDown(addr string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	idx := -1
	for i, saddr := range g.slaves {
		if saddr == addr {
			idx = i
			break
		}
	}

	if idx == -1 {
		return
	}

	g.slaves[idx] = g.slaves[len(g.slaves)-1]
	g.slaves = g.slaves[:len(g.slaves)-1]
}
