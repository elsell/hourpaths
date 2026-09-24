package path

import "errors"

var (
	ErrInvalidFields                = errors.New("domain fields are invalid")
	ErrInvalidState                 = errors.New("domain state transition is invalid")
	ErrInvitationUnavailable        = errors.New("path invitation is unavailable")
	ErrOwnershipTransferUnavailable = errors.New("path ownership transfer is unavailable")
)
