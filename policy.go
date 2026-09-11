package qac

import "context"

// Policy decides how to allocate a request.
type Policy interface {
	Decide(context.Context, Request) (Decision, error)
}
