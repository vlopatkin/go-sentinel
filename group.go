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

func (g *group) slavesAddrs() []string {
	g.mu.RLock()

	addrs := make([]string, len(g.slaves))
	copy(addrs, g.slaves)

	g.mu.RUnlock()

	return addrs
}

func (g *group) syncMaster(addr string) {
	g.mu.RLock()

	if g.master == addr {
		g.mu.RUnlock()
		return
	}

	g.mu.RUnlock()

	g.mu.Lock()
	g.master = addr

	g.mu.Unlock()
}

func (g *group) syncSlaves(addrs []string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.slaves = addrs
}

func (g *group) addSlave(addr string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for _, saddr := range g.slaves {
		if saddr == addr {
			return
		}
	}

	g.slaves = append(g.slaves, addr)
}

func (g *group) removeSlave(addr string) {
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

func (g *group) reset() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.master = ""
	g.slaves = nil
}
