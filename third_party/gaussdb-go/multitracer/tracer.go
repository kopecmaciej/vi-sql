// Package multitracer provides a Tracer that can combine several tracers into one.
package multitracer

import (
	"context"

	"github.com/HuaweiCloudDeveloper/gaussdb-go"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxpool"
)

// Tracer can combine several tracers into one.
// You can use New to automatically split tracers by interface.
type Tracer struct {
	QueryTracers       []gaussdbgo.QueryTracer
	BatchTracers       []gaussdbgo.BatchTracer
	CopyFromTracers    []gaussdbgo.CopyFromTracer
	PrepareTracers     []gaussdbgo.PrepareTracer
	ConnectTracers     []gaussdbgo.ConnectTracer
	PoolAcquireTracers []gaussdbxpool.AcquireTracer
	PoolReleaseTracers []gaussdbxpool.ReleaseTracer
}

// New returns new Tracer from tracers with automatically split tracers by interface.
func New(tracers ...gaussdbgo.QueryTracer) *Tracer {
	var t Tracer

	for _, tracer := range tracers {
		t.QueryTracers = append(t.QueryTracers, tracer)

		if batchTracer, ok := tracer.(gaussdbgo.BatchTracer); ok {
			t.BatchTracers = append(t.BatchTracers, batchTracer)
		}

		if copyFromTracer, ok := tracer.(gaussdbgo.CopyFromTracer); ok {
			t.CopyFromTracers = append(t.CopyFromTracers, copyFromTracer)
		}

		if prepareTracer, ok := tracer.(gaussdbgo.PrepareTracer); ok {
			t.PrepareTracers = append(t.PrepareTracers, prepareTracer)
		}

		if connectTracer, ok := tracer.(gaussdbgo.ConnectTracer); ok {
			t.ConnectTracers = append(t.ConnectTracers, connectTracer)
		}

		if poolAcquireTracer, ok := tracer.(gaussdbxpool.AcquireTracer); ok {
			t.PoolAcquireTracers = append(t.PoolAcquireTracers, poolAcquireTracer)
		}

		if poolReleaseTracer, ok := tracer.(gaussdbxpool.ReleaseTracer); ok {
			t.PoolReleaseTracers = append(t.PoolReleaseTracers, poolReleaseTracer)
		}
	}

	return &t
}

func (t *Tracer) TraceQueryStart(ctx context.Context, conn *gaussdbgo.Conn, data gaussdbgo.TraceQueryStartData) context.Context {
	for _, tracer := range t.QueryTracers {
		ctx = tracer.TraceQueryStart(ctx, conn, data)
	}

	return ctx
}

func (t *Tracer) TraceQueryEnd(ctx context.Context, conn *gaussdbgo.Conn, data gaussdbgo.TraceQueryEndData) {
	for _, tracer := range t.QueryTracers {
		tracer.TraceQueryEnd(ctx, conn, data)
	}
}

func (t *Tracer) TraceBatchStart(ctx context.Context, conn *gaussdbgo.Conn, data gaussdbgo.TraceBatchStartData) context.Context {
	for _, tracer := range t.BatchTracers {
		ctx = tracer.TraceBatchStart(ctx, conn, data)
	}

	return ctx
}

func (t *Tracer) TraceBatchQuery(ctx context.Context, conn *gaussdbgo.Conn, data gaussdbgo.TraceBatchQueryData) {
	for _, tracer := range t.BatchTracers {
		tracer.TraceBatchQuery(ctx, conn, data)
	}
}

func (t *Tracer) TraceBatchEnd(ctx context.Context, conn *gaussdbgo.Conn, data gaussdbgo.TraceBatchEndData) {
	for _, tracer := range t.BatchTracers {
		tracer.TraceBatchEnd(ctx, conn, data)
	}
}

func (t *Tracer) TraceCopyFromStart(ctx context.Context, conn *gaussdbgo.Conn, data gaussdbgo.TraceCopyFromStartData) context.Context {
	for _, tracer := range t.CopyFromTracers {
		ctx = tracer.TraceCopyFromStart(ctx, conn, data)
	}

	return ctx
}

func (t *Tracer) TraceCopyFromEnd(ctx context.Context, conn *gaussdbgo.Conn, data gaussdbgo.TraceCopyFromEndData) {
	for _, tracer := range t.CopyFromTracers {
		tracer.TraceCopyFromEnd(ctx, conn, data)
	}
}

func (t *Tracer) TracePrepareStart(ctx context.Context, conn *gaussdbgo.Conn, data gaussdbgo.TracePrepareStartData) context.Context {
	for _, tracer := range t.PrepareTracers {
		ctx = tracer.TracePrepareStart(ctx, conn, data)
	}

	return ctx
}

func (t *Tracer) TracePrepareEnd(ctx context.Context, conn *gaussdbgo.Conn, data gaussdbgo.TracePrepareEndData) {
	for _, tracer := range t.PrepareTracers {
		tracer.TracePrepareEnd(ctx, conn, data)
	}
}

func (t *Tracer) TraceConnectStart(ctx context.Context, data gaussdbgo.TraceConnectStartData) context.Context {
	for _, tracer := range t.ConnectTracers {
		ctx = tracer.TraceConnectStart(ctx, data)
	}

	return ctx
}

func (t *Tracer) TraceConnectEnd(ctx context.Context, data gaussdbgo.TraceConnectEndData) {
	for _, tracer := range t.ConnectTracers {
		tracer.TraceConnectEnd(ctx, data)
	}
}

func (t *Tracer) TraceAcquireStart(ctx context.Context, pool *gaussdbxpool.Pool, data gaussdbxpool.TraceAcquireStartData) context.Context {
	for _, tracer := range t.PoolAcquireTracers {
		ctx = tracer.TraceAcquireStart(ctx, pool, data)
	}

	return ctx
}

func (t *Tracer) TraceAcquireEnd(ctx context.Context, pool *gaussdbxpool.Pool, data gaussdbxpool.TraceAcquireEndData) {
	for _, tracer := range t.PoolAcquireTracers {
		tracer.TraceAcquireEnd(ctx, pool, data)
	}
}

func (t *Tracer) TraceRelease(pool *gaussdbxpool.Pool, data gaussdbxpool.TraceReleaseData) {
	for _, tracer := range t.PoolReleaseTracers {
		tracer.TraceRelease(pool, data)
	}
}
