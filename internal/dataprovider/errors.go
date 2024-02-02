package dataprovider

import (
	"errors"
	"os"
)

var (
	ErrExist         = os.ErrExist
	ErrNotExist      = os.ErrNotExist
	ErrPermission    = os.ErrPermission
	ErrInvalidParent = &os.PathError{Err: errors.New("parent does not exist or not a directory")}
)
