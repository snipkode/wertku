package mysql

import (
	"crypto/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

// newUID returns a fresh 26-char Crockford-base32 ULID.
// Used to populate the public `uid` column in users, wallets, and transactions.
func newUID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}
