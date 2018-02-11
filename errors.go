package sentinel

import (
	"errors"
)

var (
	// ErrMasterUnavailable is returned by GetMasterAddr if address is not yet discovered from sentinels
	ErrMasterUnavailable = errors.New("cannot discover master from sentinel")
	// ErrInvalidMasterName is retuned by GetMasterAddr, GetSlavesAddrs if master name (group) is not configured in watcher
	ErrInvalidMasterName = errors.New("master name not configured")

	errInvalidGetMasterAddrReply = errors.New("invalid sentinel get-master-addr-by-name reply")
	errMasterNameNotFound        = errors.New("master not found in redis sentinel")
	errInvalidRoleReply          = errors.New("invalid role reply")
)
