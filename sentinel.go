package sentinel

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
)

const (
	masterRole = "master"
	slaveRole  = "slave"
)

var (
	listenRetryTimeout = 1 * time.Second
)

// Config represents sentinel watcher config
type Config struct {
	// Addr represents redis sentinel address
	Addr string
	// Groups represents list of groups (master names) to discover
	Groups []string

	// DialTimeout represents timeout for tcp dial
	DialTimeout time.Duration
	// ReadTimeout represents timeout reading from connection
	ReadTimeout time.Duration
	// WriteTimeout represents timeout writing to connection
	WriteTimeout time.Duration

	// DiscoverInterval represents interval for redis instances discover
	DiscoverInterval time.Duration
	// DiscoverRushInterval represents interval for redis instances discover in case of error occurred
	DiscoverRushInterval time.Duration

	// HeartbeatInterval represents pub/sub conn healthcheck interval
	HeartbeatInterval time.Duration
	// HeartbeatInterval represents timeout for pub/sub conn healthcheck reply
	HeartbeatTimeout time.Duration
}

// Sentinel represents redis sentinel watcher
type Sentinel struct {
	addr   string
	groups map[string]*group

	dialTimeout  time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration

	discoverInterval     time.Duration
	discoverRushInterval time.Duration

	heartbeatInterval time.Duration
	heartbeatTimeout  time.Duration

	stop chan bool
}

// New creates sentinel watcher with provided config
func New(c *Config) *Sentinel {
	groups := make(map[string]*group, len(c.Groups))

	for _, name := range c.Groups {
		groups[name] = &group{
			name: name,
		}
	}

	return &Sentinel{
		addr:                 c.Addr,
		groups:               groups,
		dialTimeout:          c.DialTimeout,
		readTimeout:          c.ReadTimeout,
		writeTimeout:         c.WriteTimeout,
		discoverInterval:     c.DiscoverInterval,
		discoverRushInterval: c.DiscoverRushInterval,
		heartbeatInterval:    c.HeartbeatInterval,
		heartbeatTimeout:     c.HeartbeatTimeout,
	}
}

// Run starts sentinel watcher discover and pub/sub listen
func (s *Sentinel) Run() {
	s.stop = make(chan bool)

	ctx, cancel := context.WithCancel(context.Background())

	go func(ctx context.Context) {
		t := time.NewTicker(s.discoverInterval)

		for {
			select {
			case <-t.C:
				s.discover()

			case <-ctx.Done():
				t.Stop()
				return
			}
		}
	}(ctx)

	go s.listen(ctx)

	<-s.stop

	cancel()
}

func (s *Sentinel) discover() error {
	conn, err := redis.Dial("tcp", s.addr,
		redis.DialConnectTimeout(s.dialTimeout),
		redis.DialReadTimeout(s.readTimeout),
		redis.DialWriteTimeout(s.writeTimeout),
	)

	if err != nil {
		return err
	}

	defer conn.Close()

	for _, grp := range s.groups {
		s.discoverGroup(conn, grp)
	}

	return nil
}

func (s *Sentinel) discoverGroup(conn redis.Conn, grp *group) error {
	masterAddr, err := s.getMasterAddr(conn, grp.name)
	if err != nil {
		return err
	}

	err = s.testRole(masterAddr, masterRole)
	if err != nil {
		return err
	}

	grp.syncMaster(masterAddr)

	slavesAddrs, err := s.getSlavesAddrs(conn, grp.name)
	if err != nil {
		return err
	}

	grp.syncSlaves(slavesAddrs)

	return nil
}

func (s *Sentinel) listen(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			break
		}

		conn, err := redis.Dial("tcp", s.addr,
			redis.DialConnectTimeout(s.dialTimeout),
			redis.DialReadTimeout(s.heartbeatInterval+s.heartbeatTimeout),
			redis.DialWriteTimeout(s.writeTimeout),
		)

		if err != nil {
			time.Sleep(listenRetryTimeout)
			continue
		}

		psc := redis.PubSubConn{conn}

		err = psc.Subscribe("+switch-master", "+slave", "+sdown", "-sdown")
		if err != nil {
			psc.Close()
			continue
		}

		pingStop := make(chan bool, 1)

		recvDone := s.listenReceive(psc)
		pingDone := s.listenPing(psc, pingStop)

		select {
		case <-recvDone:
			close(pingStop)

			psc.Close()

		case <-pingDone:
			psc.Close()

		case <-ctx.Done():
			psc.Unsubscribe()

			close(pingStop)

			<-recvDone

			psc.Close()
			return
		}

		time.Sleep(listenRetryTimeout)
	}
}

func (s *Sentinel) listenPing(psc redis.PubSubConn, stop <-chan bool) (done chan error) {
	done = make(chan error, 1)

	go func(stop <-chan bool, done chan error) {
		t := time.NewTicker(s.heartbeatInterval)

		defer t.Stop()
		defer close(done)

		for {
			select {
			case <-t.C:
				psc.Conn.Send("PING")
				if err := psc.Conn.Flush(); err != nil {
					done <- err
					return
				}
			case <-stop:
				return
			}
		}

	}(stop, done)

	return
}

func (s *Sentinel) listenReceive(conn redis.PubSubConn) (done chan error) {
	done = make(chan error, 1)

	go func(done chan error) {
		defer close(done)

		for {
			switch v := conn.Receive().(type) {
			case redis.Subscription:
				if v.Count == 0 {
					return
				}
			case redis.Message:
				s.handleNotification(v)
			case error:
				done <- v
				return
			}
		}
	}(done)

	return
}

func (s *Sentinel) handleNotification(msg redis.Message) {
	parts := strings.Split(string(msg.Data), " ")

	switch msg.Channel {
	case "+switch-master":
		// <master name> <oldip> <oldport> <newip> <newport>
		if len(parts) != 5 {
			return
		}

		grp, ok := s.groups[parts[0]]
		if !ok {
			return
		}

		grp.syncMaster(net.JoinHostPort(parts[3], parts[4]))

	case "+slave", "+sdown", "-sdown":
		// <instance-type> <name> <ip> <port> @ <master-name> <master-ip> <master-port>
		if len(parts) != 8 {
			return
		}

		if parts[0] != slaveRole {
			return
		}

		grp, ok := s.groups[parts[5]]
		if !ok {
			return
		}

		addr := net.JoinHostPort(parts[2], parts[3])

		if msg.Channel == "+sdown" {
			grp.syncSlaveDown(addr)
			return
		}

		grp.syncSlaveUp(addr)
	}
}

func (s *Sentinel) getMasterAddr(conn redis.Conn, name string) (string, error) {
	v, err := redis.Strings(conn.Do("SENTINEL", "get-master-addr-by-name", name))
	if err != nil {
		if err == redis.ErrNil {
			return "", errMasterNotFound
		}
		return "", err
	}

	if len(v) != 2 {
		return "", errInvalidGetMasterAddrReply
	}

	return net.JoinHostPort(v[0], v[1]), nil
}

func (s *Sentinel) getSlavesAddrs(conn redis.Conn, name string) ([]string, error) {
	vals, err := redis.Values(conn.Do("SENTINEL", "slaves", name))
	if err != nil {
		return nil, err
	}

	addrs := make([]string, 0, len(vals))

	for _, v := range vals {
		slave, err := redis.StringMap(v, nil)
		if err != nil {
			return nil, err
		}

		flags := slave["flags"]

		if !strings.Contains(flags, "s_down") && !strings.Contains(flags, "disconnected") {
			addrs = append(addrs, net.JoinHostPort(slave["ip"], slave["port"]))
		}
	}

	return addrs, nil
}

func (s *Sentinel) testRole(addr, expRole string) error {
	role, err := s.getRole(addr)
	if err != nil {
		return err
	}

	if role != expRole {
		return fmt.Errorf("%s has invalid role: %s", addr, role)
	}

	return nil
}

func (s *Sentinel) getRole(addr string) (string, error) {
	conn, err := redis.Dial("tcp", addr,
		redis.DialConnectTimeout(s.dialTimeout),
		redis.DialReadTimeout(s.readTimeout),
		redis.DialWriteTimeout(s.writeTimeout),
	)

	if err != nil {
		return "", err
	}

	v, err := redis.Values(conn.Do("ROLE"))
	if err != nil {
		return "", err
	}

	if len(v) < 2 {
		return "", errInvalidRoleReply
	}

	return redis.String(v[0], nil)
}

// Stop initiates graceful shutdown of sentinel watcher.
// It blocks until all connections released
func (s *Sentinel) Stop() {
	if s.stop == nil {
		return
	}

	s.stop <- true
}
