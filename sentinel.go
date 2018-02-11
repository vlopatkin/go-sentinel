package sentinel

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/garyburd/redigo/redis"
)

const (
	masterRole = "master"
	slaveRole  = "slave"
)

var (
	defaultDiscoverInterval = 30 * time.Second

	defaultHeartbeatInterval = 15 * time.Second
	defaultHeartbeatTimeout  = 5 * time.Second

	listenRetryTimeout = 1 * time.Second
)

// Config is sentinel watcher config
type Config struct {
	// Addrs is a list of redis sentinel instances addresses
	Addrs []string
	// Groups is a list of groups (master names) to discover
	Groups []string

	// DialTimeout specifies the timeout for tcp dial
	DialTimeout time.Duration
	// ReadTimeout specifies the timeout reading from connection
	ReadTimeout time.Duration
	// WriteTimeout specifies the timeout writing to connection
	WriteTimeout time.Duration

	// DiscoverInterval specifies the interval for redis instances discovery
	DiscoverInterval time.Duration

	// HeartbeatInterval specifies the interval for pub/sub connection healthchecks
	HeartbeatInterval time.Duration
	// HeartbeatTimeout specifies the timeout reading pub/sub connection healthcheck reply
	HeartbeatTimeout time.Duration

	// OnError is the errors hook
	OnError func(err error)
}

// Sentinel is sentinel watcher
type Sentinel struct {
	addrs []string
	pos   int64

	groups map[string]*group

	dialTimeout  time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration

	discoverInterval time.Duration

	heartbeatInterval time.Duration
	heartbeatTimeout  time.Duration

	stop chan bool
	wg   sync.WaitGroup

	onError func(err error)
}

// New creates sentinel watcher with provided config
func New(c *Config) *Sentinel {
	if len(c.Addrs) < 1 {
		panic("at least 1 sentinel address is required")
	}

	groups := make(map[string]*group, len(c.Groups))

	for _, name := range c.Groups {
		groups[name] = &group{name: name}
	}

	onError := c.OnError
	// nop error hook to avoid nil checks
	if onError == nil {
		onError = func(err error) {}
	}

	discoverInterval := c.DiscoverInterval
	if discoverInterval < 1 {
		discoverInterval = defaultDiscoverInterval
	}

	heartbeatInterval := c.HeartbeatInterval
	if heartbeatInterval < 1 {
		heartbeatInterval = defaultHeartbeatInterval
	}

	heartbeatTimeout := c.HeartbeatTimeout
	if heartbeatTimeout < 1 {
		heartbeatTimeout = defaultHeartbeatTimeout
	}

	return &Sentinel{
		addrs:             c.Addrs,
		groups:            groups,
		dialTimeout:       c.DialTimeout,
		readTimeout:       c.ReadTimeout,
		writeTimeout:      c.WriteTimeout,
		discoverInterval:  discoverInterval,
		heartbeatInterval: heartbeatInterval,
		heartbeatTimeout:  heartbeatTimeout,
		onError:           onError,
	}
}

// GetMasterAddr returns redis master address
func (s *Sentinel) GetMasterAddr(name string) (string, error) {
	if grp, ok := s.groups[name]; ok {
		if addr := grp.getMaster(); addr != "" {
			return addr, nil
		}

		return "", ErrMasterUnavailable
	}

	return "", ErrInvalidMasterName
}

// GetSlavesAddrs returns reachable redis slaves addresses
func (s *Sentinel) GetSlavesAddrs(name string) ([]string, error) {
	if grp, ok := s.groups[name]; ok {
		return grp.getSlaves(), nil
	}

	return nil, ErrInvalidMasterName
}

// Run starts redis instances discovery and pub/sub updates listening
func (s *Sentinel) Run() {
	s.stop = make(chan bool)

	ctx, cancel := context.WithCancel(context.Background())

	s.wg.Add(1)
	go func(ctx context.Context) {
		defer s.wg.Done()

		s.discover()

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

	s.wg.Add(1)
	go s.listen(ctx)

	<-s.stop

	cancel()
}

// Stop initiates graceful shutdown of sentinel watcher.
// It blocks until all underlying connections are released
func (s *Sentinel) Stop() {
	if s.stop == nil {
		return
	}

	close(s.stop)
	s.wg.Wait()

	s.stop = nil
}

func (s *Sentinel) discover() {
	conn, err := redis.Dial("tcp", s.getAddr(),
		redis.DialConnectTimeout(s.dialTimeout),
		redis.DialReadTimeout(s.readTimeout),
		redis.DialWriteTimeout(s.writeTimeout),
	)

	if err != nil {
		s.shiftAddr()
		s.onError(err)
		return
	}

	for _, grp := range s.groups {
		if err := s.discoverGroup(conn, grp); err != nil {
			s.onError(fmt.Errorf("%s %s", grp.name, err))
		}
	}

	conn.Close()
}

func (s *Sentinel) discoverGroup(conn redis.Conn, grp *group) error {
	master, err := s.discoverMaster(conn, grp.name)
	if err != nil {
		return err
	}

	err = s.testRole(master, masterRole)
	if err != nil {
		return err
	}

	grp.syncMaster(master)

	slaves, err := s.discoverSlaves(conn, grp.name)
	if err != nil {
		return err
	}

	grp.syncSlaves(slaves)

	return nil
}

func (s *Sentinel) listen(ctx context.Context) {
	defer s.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			break
		}

		conn, err := redis.Dial("tcp", s.getAddr(),
			redis.DialConnectTimeout(s.dialTimeout),
			redis.DialReadTimeout(s.heartbeatInterval+s.heartbeatTimeout),
			redis.DialWriteTimeout(s.writeTimeout),
		)

		if err != nil {
			s.shiftAddr()
			s.onError(fmt.Errorf("pub/sub conn %s", err))
			time.Sleep(listenRetryTimeout)
			continue
		}

		psconn := redis.PubSubConn{conn}

		err = psconn.Subscribe("+switch-master", "+slave", "+sdown", "-sdown")
		if err != nil {
			s.onError(fmt.Errorf("pub/sub conn %s", err))
			psconn.Close()
			continue
		}

		recvDone := s.listenReceive(psconn)

		stopPing := make(chan bool, 1)
		pingDone := s.listenPing(psconn, stopPing)

		select {
		case err := <-recvDone:
			if err != nil {
				s.onError(fmt.Errorf("pub/sub conn receive %s", err))
			}

			close(stopPing)

			psconn.Close()

		case err := <-pingDone:
			if err != nil {
				s.onError(fmt.Errorf("pub/sub conn ping %s", err))
			}

			psconn.Close()

		case <-ctx.Done():
			psconn.Unsubscribe()

			close(stopPing)

			<-recvDone

			psconn.Close()
			return
		}

		time.Sleep(listenRetryTimeout)
	}
}

func (s *Sentinel) listenPing(psconn redis.PubSubConn, stop <-chan bool) (done chan error) {
	done = make(chan error, 1)

	go func(stop <-chan bool, done chan error) {
		t := time.NewTicker(s.heartbeatInterval)

		defer close(done)
		defer t.Stop()

		for {
			select {
			case <-t.C:
				psconn.Conn.Send("PING")
				if err := psconn.Conn.Flush(); err != nil {
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

func (s *Sentinel) listenReceive(psconn redis.PubSubConn) (done chan error) {
	done = make(chan error, 1)

	go func(done chan error) {
		defer close(done)

		for {
			switch v := psconn.Receive().(type) {
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

func (s *Sentinel) discoverMaster(conn redis.Conn, name string) (string, error) {
	reply, err := redis.Strings(conn.Do("SENTINEL", "get-master-addr-by-name", name))
	if err != nil {
		if err == redis.ErrNil {
			return "", errMasterNameNotFound
		}
		return "", err
	}

	if len(reply) != 2 {
		return "", errInvalidGetMasterAddrReply
	}

	return net.JoinHostPort(reply[0], reply[1]), nil
}

func (s *Sentinel) discoverSlaves(conn redis.Conn, name string) ([]string, error) {
	reply, err := redis.Values(conn.Do("SENTINEL", "slaves", name))
	if err != nil {
		return nil, err
	}

	addrs := make([]string, 0, len(reply))

	for _, vals := range reply {
		slave, err := redis.StringMap(vals, nil)
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

	reply, err := redis.Values(conn.Do("ROLE"))
	if err != nil {
		return "", err
	}

	if len(reply) < 2 {
		return "", errInvalidRoleReply
	}

	return redis.String(reply[0], nil)
}

func (s *Sentinel) getAddr() string {
	return s.addrs[atomic.LoadInt64(&s.pos)]
}

func (s *Sentinel) shiftAddr() {
	atomic.StoreInt64(&s.pos, (atomic.LoadInt64(&s.pos)+1)%int64(len(s.addrs)))
}
