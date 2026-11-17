package redis

import "errors"

// errMalformedReply reports a hit script reply of an unexpected shape.
var errMalformedReply = errors.New("malformed rate limit reply")
