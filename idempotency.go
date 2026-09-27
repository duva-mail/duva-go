package duva

import (
	"crypto/rand"
	"fmt"
)

// newIdempotencyKey generates a UUID v4, used for Messages.Send when the caller doesn't supply
// one and retries are enabled: a retry after a timed-out attempt can then never create a
// duplicate (the server replays the first answer). Documented caveat: a key generated per call
// does not de-duplicate BETWEEN two calls; to de-duplicate a logical send (an order), pass a
// stable key ("order-4821").
func newIdempotencyKey() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// validateIdempotencyKey rejects a caller-supplied key locally (a clear error) rather than
// letting the server answer 422: 1 to 255 printable ASCII characters, no spaces.
func validateIdempotencyKey(key string) error {
	if len(key) < 1 || len(key) > 255 {
		return fmt.Errorf("duva: idempotency key must be 1 to 255 characters")
	}
	for _, r := range key {
		if r < 0x21 || r > 0x7e {
			return fmt.Errorf("duva: idempotency key must be printable ASCII without spaces")
		}
	}
	return nil
}
