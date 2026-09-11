package mysql

import (
	"errors"

	"github.com/go-sql-driver/mysql"
)

const (
	mysqlErrDuplicateEntry  uint16 = 1062
	mysqlErrDeadlock        uint16 = 1213
	mysqlErrLockWaitTimeout uint16 = 1205
)

// isDuplicateKeyError returns true for MySQL duplicate key violations.
func isDuplicateKeyError(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == mysqlErrDuplicateEntry
}

// IsDeadlockError returns true for MySQL deadlock errors (1213)
// or lock wait timeout errors (1205).
// Used by the retry mechanism in transfer_service.
func IsDeadlockError(err error) bool {
	var me *mysql.MySQLError
	if errors.As(err, &me) {
		return me.Number == mysqlErrDeadlock || me.Number == mysqlErrLockWaitTimeout
	}
	return false
}
