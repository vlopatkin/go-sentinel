package sentinel

import (
	"errors"
)

var (
	// master address is not yet discovered from sentinels
	ErrMasterUnavailable = errors.New("cannot discover master from sentinel")
	// master name not found in sentinel
	ErrMasterNotFound = errors.New("master not found in redis sentinel")
	// malformed redis sentinel get-master-addr-by-name command reply
	ErrInvalidGetMasterAddrReply = errors.New("invalid sentinel get-master-addr-by-name reply")
	// malformed redis ROLE command reply
	ErrInvalidRoleReply = errors.New("invalid role reply")
	// master name (group) not configured in watcher
	ErrInvalidMasterName = errors.New("master name not configured")
)
