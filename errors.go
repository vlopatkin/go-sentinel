package sentinel

import (
	"errors"
)

var (
	ErrMasterUnavailable         = errors.New("cannot discover master from sentinel")
	ErrMasterNotFound            = errors.New("master not found in redis sentinel")
	ErrInvalidGetMasterAddrReply = errors.New("invalid sentinel get-master-addr-by-name reply")
	ErrInvalidRoleReply          = errors.New("invalid role reply")
	ErrInvalidMasterName         = errors.New("master name not configured")
)
