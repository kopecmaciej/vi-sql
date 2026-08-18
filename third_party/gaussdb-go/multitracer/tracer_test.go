package multitracer_test

import (
	"context"
	"testing"

	"github.com/HuaweiCloudDeveloper/gaussdb-go"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxpool"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/multitracer"
	"github.com/stretchr/testify/require"
)

type testFullTracer struct{}

func (tt *testFullTracer) TraceQueryStart(ctx context.Context, conn *gaussdbgo.Conn, data gaussdbgo.TraceQueryStartData) context.Context {
	return ctx
}

func (tt *testFullTracer) TraceQueryEnd(ctx context.Context, conn *gaussdbgo.Conn, data gaussdbgo.TraceQueryEndData) {
}

func (tt *testFullTracer) TraceBatchStart(ctx context.Context, conn *gaussdbgo.Conn, data gaussdbgo.TraceBatchStartData) context.Context {
	return ctx
}

func (tt *testFullTracer) TraceBatchQuery(ctx context.Context, conn *gaussdbgo.Conn, data gaussdbgo.TraceBatchQueryData) {
}

func (tt *testFullTracer) TraceBatchEnd(ctx context.Context, conn *gaussdbgo.Conn, data gaussdbgo.TraceBatchEndData) {
}

func (tt *testFullTracer) TraceCopyFromStart(ctx context.Context, conn *gaussdbgo.Conn, data gaussdbgo.TraceCopyFromStartData) context.Context {
	return ctx
}

func (tt *testFullTracer) TraceCopyFromEnd(ctx context.Context, conn *gaussdbgo.Conn, data gaussdbgo.TraceCopyFromEndData) {
}

func (tt *testFullTracer) TracePrepareStart(ctx context.Context, conn *gaussdbgo.Conn, data gaussdbgo.TracePrepareStartData) context.Context {
	return ctx
}

func (tt *testFullTracer) TracePrepareEnd(ctx context.Context, conn *gaussdbgo.Conn, data gaussdbgo.TracePrepareEndData) {
}

func (tt *testFullTracer) TraceConnectStart(ctx context.Context, data gaussdbgo.TraceConnectStartData) context.Context {
	return ctx
}

func (tt *testFullTracer) TraceConnectEnd(ctx context.Context, data gaussdbgo.TraceConnectEndData) {
}

func (tt *testFullTracer) TraceAcquireStart(ctx context.Context, pool *gaussdbxpool.Pool, data gaussdbxpool.TraceAcquireStartData) context.Context {
	return ctx
}

func (tt *testFullTracer) TraceAcquireEnd(ctx context.Context, pool *gaussdbxpool.Pool, data gaussdbxpool.TraceAcquireEndData) {
}

func (tt *testFullTracer) TraceRelease(pool *gaussdbxpool.Pool, data gaussdbxpool.TraceReleaseData) {
}

type testCopyTracer struct{}

func (tt *testCopyTracer) TraceQueryStart(ctx context.Context, conn *gaussdbgo.Conn, data gaussdbgo.TraceQueryStartData) context.Context {
	return ctx
}

func (tt *testCopyTracer) TraceQueryEnd(ctx context.Context, conn *gaussdbgo.Conn, data gaussdbgo.TraceQueryEndData) {
}

func (tt *testCopyTracer) TraceCopyFromStart(ctx context.Context, conn *gaussdbgo.Conn, data gaussdbgo.TraceCopyFromStartData) context.Context {
	return ctx
}

func (tt *testCopyTracer) TraceCopyFromEnd(ctx context.Context, conn *gaussdbgo.Conn, data gaussdbgo.TraceCopyFromEndData) {
}

func TestNew(t *testing.T) {
	t.Parallel()

	fullTracer := &testFullTracer{}
	copyTracer := &testCopyTracer{}

	mt := multitracer.New(fullTracer, copyTracer)
	require.Equal(
		t,
		&multitracer.Tracer{
			QueryTracers: []gaussdbgo.QueryTracer{
				fullTracer,
				copyTracer,
			},
			BatchTracers: []gaussdbgo.BatchTracer{
				fullTracer,
			},
			CopyFromTracers: []gaussdbgo.CopyFromTracer{
				fullTracer,
				copyTracer,
			},
			PrepareTracers: []gaussdbgo.PrepareTracer{
				fullTracer,
			},
			ConnectTracers: []gaussdbgo.ConnectTracer{
				fullTracer,
			},
			PoolAcquireTracers: []gaussdbxpool.AcquireTracer{
				fullTracer,
			},
			PoolReleaseTracers: []gaussdbxpool.ReleaseTracer{
				fullTracer,
			},
		},
		mt,
	)
}
