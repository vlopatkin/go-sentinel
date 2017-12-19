package sentinel

import (
	"errors"
)

var (
	masterUnavailableErr         = errors.New("cannot discover master from sentinel")
	masterNotFoundErr            = errors.New("master not found in redis sentinel")
	invalidGetMasterAddrReplyErr = errors.New("invalid sentinel get-master-addr-by-name reply")
	invalidRoleReplyErr          = errors.New("invalid role reply")
	invalidMasterGroupErr        = errors.New("group not configured in sentinel client")
)
