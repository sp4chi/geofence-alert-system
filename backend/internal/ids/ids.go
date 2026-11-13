// Package ids generates prefixed, time-sortable IDs such as "geo_01jcz3k8m6q2v9x4t7r5w0n1bd".
//
// Layout (ULID-style): 48 bits of Unix milliseconds followed by 80 random bits, base32-encoded.
// The prefix makes logs and API responses self-describing; the time prefix keeps IDs roughly
// in creation order; the random part means IDs don't reveal how many rows exist.
package ids

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/binary"
	"time"
)

const (
	Geofence   = "geo"
	Vehicle    = "veh"
	AlertRule  = "alert"
	Violation  = "viol"
	AlertEvent = "evt"
)

// Crockford's alphabet (no i, l, o, u) in ascending ASCII order, so string order matches time order.
var encoding = base32.NewEncoding("0123456789abcdefghjkmnpqrstvwxyz").WithPadding(base32.NoPadding)

// New returns prefix + "_" + 26 base32 characters.
func New(prefix string) string {
	var raw [16]byte
	// Shift left by 16 so the 48-bit millisecond count fills bytes 0–5; bytes 6–15 become random.
	binary.BigEndian.PutUint64(raw[:8], uint64(time.Now().UnixMilli())<<16)
	_, _ = rand.Read(raw[6:]) // crypto/rand.Read never returns an error since Go 1.24
	return prefix + "_" + encoding.EncodeToString(raw[:])
}
