package core

import "errors"

// ErrRuntimeLocked indicates another process currently holds a Mihosh runtime/config lock file.
var ErrRuntimeLocked = errors.New("another Mihosh process owns this managed core")
