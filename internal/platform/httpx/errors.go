package httpx

import "errors"

// errPanic is what a recovered panic answers: a 500, its value in the log only.
var errPanic = errors.New("httpx: a handler panicked")
