package gaussdbxpool

import (
	"context"

	"github.com/HuaweiCloudDeveloper/gaussdb-go"
)

// AcquireTracer traces Acquire.
type AcquireTracer interface {
	// TraceAcquireStart is called at the beginning of Acquire.
	// The returned context is used for the rest of the call and will be passed to the TraceAcquireEnd.
	TraceAcquireStart(ctx context.Context, pool *Pool, data TraceAcquireStartData) context.Context
	// TraceAcquireEnd is called when a connection has been acquired.
	TraceAcquireEnd(ctx context.Context, pool *Pool, data TraceAcquireEndData)
}

type TraceAcquireStartData struct{}

type TraceAcquireEndData struct {
	Conn *gaussdbgo.Conn
	Err  error
}

// ReleaseTracer traces Release.
type ReleaseTracer interface {
	// TraceRelease is called at the beginning of Release.
	TraceRelease(pool *Pool, data TraceReleaseData)
}

type TraceReleaseData struct {
	Conn *gaussdbgo.Conn
}
