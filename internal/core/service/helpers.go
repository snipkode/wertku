package service

import "strconv"

// itoa converts an int64 to its decimal string representation.
// Used to build ResourceID strings for audit log entries.
func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
