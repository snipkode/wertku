package mysql

import (
	"errors"

	"github.com/go-sql-driver/mysql"
)

// isMySQLDuplicateError returns true when err is MySQL error 1062 (ER_DUP_ENTRY),
// which signals a UNIQUE constraint violation.
func isMySQLDuplicateError(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}
