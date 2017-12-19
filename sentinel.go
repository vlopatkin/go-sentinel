package sentinel

import (
	"time"
)

const (
	masterRole = "master"
	slaveRole  = "slave"
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
	// DiscoverRushInterval represents interval for redis instances discover in case of error occured
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
