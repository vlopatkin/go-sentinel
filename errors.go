package sentinel

import (
	"errors"
	"fmt"
)

var (
	errMasterUnavailable         = errors.New("cannot discover master from sentinel")
	errMasterNotFound            = errors.New("master not found in redis sentinel")
	errInvalidGetMasterAddrReply = errors.New("invalid sentinel get-master-addr-by-name reply")
	errInvalidRoleReply          = errors.New("invalid role reply")
	errInvalidMasterName         = errors.New("master name not configured")
)

type RunError struct {
	title string
	err   error
}

func (e *RunError) Error() string {
	return fmt.Sprintf("%s: %s", e.title, e.err.Error())
}
