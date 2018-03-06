package sentinel

import (
	"errors"
)

var (
	// ErrMasterUnavailable is returned by GetMasterAddr if address is not discovered
	ErrMasterUnavailable = errors.New("cannot discover master from sentinel")
	// ErrInvalidMasterName is retuned by GetMasterAddr, GetSlavesAddrs if master name (group) is not configured
	ErrInvalidMasterName = errors.New("invalid master name")

	errInvalidGetMasterAddrReply = errors.New("invalid get-master-addr-by-name reply")
	errMasterNameNotFound        = errors.New("master not found in redis sentinel")
	errInvalidRoleReply          = errors.New("invalid role reply")
)
