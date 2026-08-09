package data

import (
	"errors"
	"strings"
)

// IsTransientSQLiteError reports lock/busy failures that must be retried, never treated as success.
func IsTransientSQLiteError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errSQLiteBusySentinel) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "SQLITE_BUSY") ||
		strings.Contains(s, "database is locked") ||
		strings.Contains(s, "SQLITE_LOCKED")
}

// errSQLiteBusySentinel allows tests to inject a busy failure without a live lock.
var errSQLiteBusySentinel = errors.New("database is locked (5) (SQLITE_BUSY)")
