// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpexporter

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	arrowpb "github.com/f5/otel-arrow-adapter/api/experimental/arrow/v1"
	arrowpbMock "github.com/f5/otel-arrow-adapter/api/experimental/arrow/v1/mock"
	arrowRecord "github.com/f5/otel-arrow-adapter/pkg/otel/arrow_record"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/net/http2/hpack"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/f5/otel-arrow-adapter/collector/gen/exporter/otlpexporter/internal/arrow/grpcmock"
	"github.com/f5/otel-arrow-adapter/collector/gen/internal/testdata"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/auth"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

type mockReceiver struct {
	srv          *grpc.Server
	ln           net.Listener
	requestCount *atomic.Int32
	totalItems   *atomic.Int32
	mux          sync.Mutex
	metadata     metadata.MD
	exportError  error
}

func (r *mockReceiver) getMetadata() metadata.MD {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.metadata
}

func (r *mockReceiver) setExportError(err error) {
	r.mux.Lock()
	defer r.mux.Unlock()
	r.exportError = err
}

type mockTracesReceiver struct {
	ptraceotlp.UnimplementedGRPCServer
	mockReceiver
	lastRequest ptrace.Traces
}

func (r *mockTracesReceiver) Export(ctx context.Context, req ptraceotlp.ExportRequest) (ptraceotlp.ExportResponse, error) {
	r.requestCount.Add(int32(1))
	td := req.Traces()
	r.totalItems.Add(int32(td.SpanCount()))
	r.mux.Lock()
	defer r.mux.Unlock()
	r.lastRequest = td
	r.metadata, _ = metadata.FromIncomingContext(ctx)
	return ptraceotlp.NewExportResponse(), r.exportError
}

func (r *mockTracesReceiver) getLastRequest() ptrace.Traces {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.lastRequest
}

func otlpTracesReceiverOnGRPCServer(ln net.Listener, useTLS bool) (*mockTracesReceiver, error) {
	sopts := []grpc.ServerOption{}

	if useTLS {
		_, currentFile, _, _ := runtime.Caller(0)
		basepath := filepath.Dir(currentFile)
		certpath := filepath.Join(basepath, filepath.Join("testdata", "test_cert.pem"))
		keypath := filepath.Join(basepath, filepath.Join("testdata", "test_key.pem"))

		creds, err := credentials.NewServerTLSFromFile(certpath, keypath)
		if err != nil {
			return nil, err
		}
		sopts = append(sopts, grpc.Creds(creds))
	}

	rcv := &mockTracesReceiver{
		mockReceiver: mockReceiver{
			srv:          grpc.NewServer(sopts...),
			ln:           ln,
			requestCount: &atomic.Int32{},
			totalItems:   &atomic.Int32{},
		},
	}

	ptraceotlp.RegisterGRPCServer(rcv.srv, rcv)

	return rcv, nil
}

func (r *mockTracesReceiver) start() {
	go func() {
		_ = r.srv.Serve(r.ln)
	}()
}

type mockLogsReceiver struct {
	plogotlp.UnimplementedGRPCServer
	mockReceiver
	lastRequest plog.Logs
}

func (r *mockLogsReceiver) Export(ctx context.Context, req plogotlp.ExportRequest) (plogotlp.ExportResponse, error) {
	r.requestCount.Add(int32(1))
	ld := req.Logs()
	r.totalItems.Add(int32(ld.LogRecordCount()))
	r.mux.Lock()
	defer r.mux.Unlock()
	r.lastRequest = ld
	r.metadata, _ = metadata.FromIncomingContext(ctx)
	return plogotlp.NewExportResponse(), r.exportError
}

func (r *mockLogsReceiver) getLastRequest() plog.Logs {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.lastRequest
}

func otlpLogsReceiverOnGRPCServer(ln net.Listener) *mockLogsReceiver {
	rcv := &mockLogsReceiver{
		mockReceiver: mockReceiver{
			srv:          grpc.NewServer(),
			requestCount: &atomic.Int32{},
			totalItems:   &atomic.Int32{},
		},
	}

	// Now run it as a gRPC server
	plogotlp.RegisterGRPCServer(rcv.srv, rcv)
	go func() {
		_ = rcv.srv.Serve(ln)
	}()

	return rcv
}

type mockMetricsReceiver struct {
	pmetricotlp.UnimplementedGRPCServer
	mockReceiver
	lastRequest pmetric.Metrics
}

func (r *mockMetricsReceiver) Export(ctx context.Context, req pmetricotlp.ExportRequest) (pmetricotlp.ExportResponse, error) {
	md := req.Metrics()
	r.requestCount.Add(int32(1))
	r.totalItems.Add(int32(md.DataPointCount()))
	r.mux.Lock()
	defer r.mux.Unlock()
	r.lastRequest = md
	r.metadata, _ = metadata.FromIncomingContext(ctx)
	return pmetricotlp.NewExportResponse(), r.exportError
}

func (r *mockMetricsReceiver) getLastRequest() pmetric.Metrics {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.lastRequest
}

func otlpMetricsReceiverOnGRPCServer(ln net.Listener) *mockMetricsReceiver {
	rcv := &mockMetricsReceiver{
		mockReceiver: mockReceiver{
			srv:          grpc.NewServer(),
			requestCount: &atomic.Int32{},
			totalItems:   &atomic.Int32{},
		},
	}

	// Now run it as a gRPC server
	pmetricotlp.RegisterGRPCServer(rcv.srv, rcv)
	go func() {
		_ = rcv.srv.Serve(ln)
	}()

	return rcv
}

type hostWithExtensions struct {
	component.Host
	exts map[component.ID]component.Component
}

func newHostWithExtensions(exts map[component.ID]component.Component) component.Host {
	return &hostWithExtensions{
		Host: componenttest.NewNopHost(),
		exts: exts,
	}
}

func (h *hostWithExtensions) GetExtensions() map[component.ID]component.Component {
	return h.exts
}

type testAuthExtension struct {
	extension.Extension

	prc credentials.PerRPCCredentials
}

func newTestAuthExtension(t *testing.T, mdf func(ctx context.Context) map[string]string) auth.Client {
	ctrl := gomock.NewController(t)
	prc := grpcmock.NewMockPerRPCCredentials(ctrl)
	prc.EXPECT().RequireTransportSecurity().AnyTimes().Return(false)
	prc.EXPECT().GetRequestMetadata(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(
		func(ctx context.Context, _ ...string) (map[string]string, error) {
			return mdf(ctx), nil
		},
	)
	return &testAuthExtension{
		prc: prc,
	}
}

func (a *testAuthExtension) RoundTripper(_ http.RoundTripper) (http.RoundTripper, error) {
	return nil, fmt.Errorf("unused")
}

func (a *testAuthExtension) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	return a.prc, nil
}

func TestSendTraces(t *testing.T) {
	// Start an OTLP-compatible receiver.
	ln, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)
	rcv, _ := otlpTracesReceiverOnGRPCServer(ln, false)
	rcv.start()
	// Also closes the connection.
	defer rcv.srv.GracefulStop()

	// Start an OTLP exporter and point to the receiver.
	factory := NewFactory()
	authID := component.NewID("testauth")
	expectedHeader := []string{"header-value"}

	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.GRPCClientSettings = configgrpc.GRPCClientSettings{
		Endpoint: ln.Addr().String(),
		TLSSetting: configtls.TLSClientSetting{
			Insecure: true,
		},
		Headers: map[string]configopaque.String{
			"header": configopaque.String(expectedHeader[0]),
		},
		Auth: &configauth.Authentication{
			AuthenticatorID: authID,
		},
	}
	// This test fails w/ Arrow enabled because the function
	// passed to newTestAuthExtension() below requires it the
	// caller's context, and the newStream doesn't have it.
	cfg.Arrow.Disabled = true

	set := exportertest.NewNopCreateSettings()
	set.BuildInfo.Description = "Collector"
	set.BuildInfo.Version = "1.2.3test"
	exp, err := factory.CreateTracesExporter(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)

	defer func() {
		assert.NoError(t, exp.Shutdown(context.Background()))
	}()

	host := newHostWithExtensions(
		map[component.ID]component.Component{
			authID: newTestAuthExtension(t, func(ctx context.Context) map[string]string {
				return map[string]string{
					"callerid": client.FromContext(ctx).Metadata.Get("in_callerid")[0],
				}
			}),
		},
	)
	assert.NoError(t, exp.Start(context.Background(), host))

	// Ensure that initially there is no data in the receiver.
	assert.EqualValues(t, 0, rcv.requestCount.Load())

	newCallerContext := func(value string) context.Context {
		return client.NewContext(context.Background(),
			client.Info{
				Metadata: client.NewMetadata(map[string][]string{
					"in_callerid": {value},
				}),
			},
		)
	}
	const caller1 = "caller1"
	const caller2 = "caller2"
	callCtx1 := newCallerContext(caller1)
	callCtx2 := newCallerContext(caller2)

	// Send empty trace.
	td := ptrace.NewTraces()
	assert.NoError(t, exp.ConsumeTraces(callCtx1, td))

	// Wait until it is received.
	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() > 0
	}, 10*time.Second, 5*time.Millisecond)

	// Ensure it was received empty.
	assert.EqualValues(t, 0, rcv.totalItems.Load())
	md := rcv.getMetadata()

	// Expect caller1 and the static header
	require.EqualValues(t, expectedHeader, md.Get("header"))
	require.EqualValues(t, []string{caller1}, md.Get("callerid"))

	// A trace with 2 spans.
	td = testdata.GenerateTraces(2)

	err = exp.ConsumeTraces(callCtx2, td)
	assert.NoError(t, err)

	// Wait until it is received.
	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() > 1
	}, 10*time.Second, 5*time.Millisecond)

	// Verify received span.
	assert.EqualValues(t, 2, rcv.totalItems.Load())
	assert.EqualValues(t, 2, rcv.requestCount.Load())
	assert.EqualValues(t, td, rcv.getLastRequest())

	// Test the static metadata
	md = rcv.getMetadata()
	require.EqualValues(t, expectedHeader, md.Get("header"))
	require.Equal(t, len(md.Get("User-Agent")), 1)
	require.Contains(t, md.Get("User-Agent")[0], "Collector/1.2.3test")

	// Test the caller's dynamic metadata
	require.EqualValues(t, []string{caller2}, md.Get("callerid"))
}

func TestSendTracesWhenEndpointHasHttpScheme(t *testing.T) {
	tests := []struct {
		name               string
		useTLS             bool
		scheme             string
		gRPCClientSettings configgrpc.GRPCClientSettings
	}{
		{
			name:               "Use https scheme",
			useTLS:             true,
			scheme:             "https://",
			gRPCClientSettings: configgrpc.GRPCClientSettings{},
		},
		{
			name:   "Use http scheme",
			useTLS: false,
			scheme: "http://",
			gRPCClientSettings: configgrpc.GRPCClientSettings{
				TLSSetting: configtls.TLSClientSetting{
					Insecure: true,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Start an OTLP-compatible receiver.
			ln, err := net.Listen("tcp", "localhost:")
			require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)
			rcv, err := otlpTracesReceiverOnGRPCServer(ln, test.useTLS)
			rcv.start()
			require.NoError(t, err, "Failed to start mock OTLP receiver")
			// Also closes the connection.
			defer rcv.srv.GracefulStop()

			// Start an OTLP exporter and point to the receiver.
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.GRPCClientSettings = test.gRPCClientSettings
			cfg.GRPCClientSettings.Endpoint = test.scheme + ln.Addr().String()
			if test.useTLS {
				cfg.GRPCClientSettings.TLSSetting.InsecureSkipVerify = true
			}
			set := exportertest.NewNopCreateSettings()
			exp, err := factory.CreateTracesExporter(context.Background(), set, cfg)
			require.NoError(t, err)
			require.NotNil(t, exp)

			defer func() {
				assert.NoError(t, exp.Shutdown(context.Background()))
			}()

			host := componenttest.NewNopHost()
			assert.NoError(t, exp.Start(context.Background(), host))

			// Ensure that initially there is no data in the receiver.
			assert.EqualValues(t, 0, rcv.requestCount.Load())

			// Send empty trace.
			td := ptrace.NewTraces()
			assert.NoError(t, exp.ConsumeTraces(context.Background(), td))

			// Wait until it is received.
			assert.Eventually(t, func() bool {
				return rcv.requestCount.Load() > 0
			}, 10*time.Second, 5*time.Millisecond)

			// Ensure it was received empty.
			assert.EqualValues(t, 0, rcv.totalItems.Load())
		})
	}
}

func TestSendMetrics(t *testing.T) {
	// Start an OTLP-compatible receiver.
	ln, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)
	rcv := otlpMetricsReceiverOnGRPCServer(ln)
	// Also closes the connection.
	defer rcv.srv.GracefulStop()

	// Start an OTLP exporter and point to the receiver.
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.GRPCClientSettings = configgrpc.GRPCClientSettings{
		Endpoint: ln.Addr().String(),
		TLSSetting: configtls.TLSClientSetting{
			Insecure: true,
		},
		Headers: map[string]configopaque.String{
			"header": "header-value",
		},
	}
	set := exportertest.NewNopCreateSettings()
	set.BuildInfo.Description = "Collector"
	set.BuildInfo.Version = "1.2.3test"
	exp, err := factory.CreateMetricsExporter(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)
	defer func() {
		assert.NoError(t, exp.Shutdown(context.Background()))
	}()

	host := componenttest.NewNopHost()

	assert.NoError(t, exp.Start(context.Background(), host))

	// Ensure that initially there is no data in the receiver.
	assert.EqualValues(t, 0, rcv.requestCount.Load())

	// Send empty metric.
	md := pmetric.NewMetrics()
	assert.NoError(t, exp.ConsumeMetrics(context.Background(), md))

	// Wait until it is received.
	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() > 0
	}, 10*time.Second, 5*time.Millisecond)

	// Ensure it was received empty.
	assert.EqualValues(t, 0, rcv.totalItems.Load())

	// Send two metrics.
	md = testdata.GenerateMetrics(2)

	err = exp.ConsumeMetrics(context.Background(), md)
	assert.NoError(t, err)

	// Wait until it is received.
	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() > 1
	}, 10*time.Second, 5*time.Millisecond)

	expectedHeader := []string{"header-value"}

	// Verify received metrics.
	assert.EqualValues(t, 2, rcv.requestCount.Load())
	assert.EqualValues(t, 4, rcv.totalItems.Load())
	assert.EqualValues(t, md, rcv.getLastRequest())

	mdata := rcv.getMetadata()
	require.EqualValues(t, mdata.Get("header"), expectedHeader)
	require.Equal(t, len(mdata.Get("User-Agent")), 1)
	require.Contains(t, mdata.Get("User-Agent")[0], "Collector/1.2.3test")
}

func TestSendTraceDataServerDownAndUp(t *testing.T) {
	// Find the addr, but don't start the server.
	ln, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)

	// Start an OTLP exporter and point to the receiver.
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	// Disable queuing to ensure that we execute the request when calling ConsumeTraces
	// otherwise we will not see the error.
	cfg.QueueSettings.Enabled = false
	cfg.GRPCClientSettings = configgrpc.GRPCClientSettings{
		Endpoint: ln.Addr().String(),
		TLSSetting: configtls.TLSClientSetting{
			Insecure: true,
		},
		// Need to wait for every request blocking until either request timeouts or succeed.
		// Do not rely on external retry logic here, if that is intended set InitialInterval to 100ms.
		WaitForReady: true,
	}
	set := exportertest.NewNopCreateSettings()
	exp, err := factory.CreateTracesExporter(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)
	defer func() {
		assert.NoError(t, exp.Shutdown(context.Background()))
	}()

	host := componenttest.NewNopHost()

	assert.NoError(t, exp.Start(context.Background(), host))

	// A trace with 2 spans.
	td := testdata.GenerateTraces(2)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	assert.Error(t, exp.ConsumeTraces(ctx, td))
	assert.EqualValues(t, context.DeadlineExceeded, ctx.Err())
	cancel()

	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Second)
	assert.Error(t, exp.ConsumeTraces(ctx, td))
	assert.EqualValues(t, context.DeadlineExceeded, ctx.Err())
	cancel()

	startServerAndMakeRequest(t, exp, td, ln)

	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Second)
	assert.Error(t, exp.ConsumeTraces(ctx, td))
	assert.EqualValues(t, context.DeadlineExceeded, ctx.Err())
	cancel()

	// First call to startServerAndMakeRequest closed the connection. There is a race condition here that the
	// port may be reused, if this gets flaky rethink what to do.
	ln, err = net.Listen("tcp", ln.Addr().String())
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)
	startServerAndMakeRequest(t, exp, td, ln)

	ctx, cancel = context.WithTimeout(context.Background(), 1*time.Second)
	assert.Error(t, exp.ConsumeTraces(ctx, td))
	assert.EqualValues(t, context.DeadlineExceeded, ctx.Err())
	cancel()
}

func TestSendTraceDataServerStartWhileRequest(t *testing.T) {
	// Find the addr, but don't start the server.
	ln, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)

	// Start an OTLP exporter and point to the receiver.
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.GRPCClientSettings = configgrpc.GRPCClientSettings{
		Endpoint: ln.Addr().String(),
		TLSSetting: configtls.TLSClientSetting{
			Insecure: true,
		},
	}
	set := exportertest.NewNopCreateSettings()
	exp, err := factory.CreateTracesExporter(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)
	defer func() {
		assert.NoError(t, exp.Shutdown(context.Background()))
	}()

	host := componenttest.NewNopHost()

	assert.NoError(t, exp.Start(context.Background(), host))

	// A trace with 2 spans.
	td := testdata.GenerateTraces(2)
	done := make(chan bool, 1)
	defer close(done)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	go func() {
		assert.NoError(t, exp.ConsumeTraces(ctx, td))
		done <- true
	}()

	time.Sleep(2 * time.Second)
	rcv, _ := otlpTracesReceiverOnGRPCServer(ln, false)
	rcv.start()
	defer rcv.srv.GracefulStop()
	// Wait until one of the conditions below triggers.
	select {
	case <-ctx.Done():
		t.Fail()
	case <-done:
		assert.NoError(t, ctx.Err())
	}
	cancel()
}

func TestSendTracesOnResourceExhaustion(t *testing.T) {
	ln, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err)
	rcv, _ := otlpTracesReceiverOnGRPCServer(ln, false)
	rcv.setExportError(status.Error(codes.ResourceExhausted, "resource exhausted"))
	rcv.start()
	defer rcv.srv.GracefulStop()

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.RetrySettings.InitialInterval = 0
	cfg.GRPCClientSettings = configgrpc.GRPCClientSettings{
		Endpoint: ln.Addr().String(),
		TLSSetting: configtls.TLSClientSetting{
			Insecure: true,
		},
	}
	set := exportertest.NewNopCreateSettings()
	exp, err := factory.CreateTracesExporter(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)

	defer func() {
		assert.NoError(t, exp.Shutdown(context.Background()))
	}()

	host := componenttest.NewNopHost()
	assert.NoError(t, exp.Start(context.Background(), host))

	assert.EqualValues(t, 0, rcv.requestCount.Load())

	td := ptrace.NewTraces()
	assert.NoError(t, exp.ConsumeTraces(context.Background(), td))

	assert.Never(t, func() bool {
		return rcv.requestCount.Load() > 1
	}, 1*time.Second, 5*time.Millisecond, "Should not retry if RetryInfo is not included into status details by the server.")

	rcv.requestCount.Swap(0)

	st := status.New(codes.ResourceExhausted, "resource exhausted")
	st, _ = st.WithDetails(&errdetails.RetryInfo{
		RetryDelay: durationpb.New(100 * time.Millisecond),
	})
	rcv.setExportError(st.Err())

	assert.NoError(t, exp.ConsumeTraces(context.Background(), td))

	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() > 1
	}, 10*time.Second, 5*time.Millisecond, "Should retry if RetryInfo is included into status details by the server.")
}

func startServerAndMakeRequest(t *testing.T, exp exporter.Traces, td ptrace.Traces, ln net.Listener) {
	rcv, _ := otlpTracesReceiverOnGRPCServer(ln, false)
	rcv.start()
	defer rcv.srv.GracefulStop()
	// Ensure that initially there is no data in the receiver.
	assert.EqualValues(t, 0, rcv.requestCount.Load())

	// Clone the request and store as expected.
	expectedData := ptrace.NewTraces()
	td.CopyTo(expectedData)

	// Resend the request, this should succeed.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	assert.NoError(t, exp.ConsumeTraces(ctx, td))
	cancel()

	// Wait until it is received.
	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() > 0
	}, 10*time.Second, 5*time.Millisecond)

	// Verify received span.
	assert.EqualValues(t, 2, rcv.totalItems.Load())
	assert.EqualValues(t, expectedData, rcv.getLastRequest())
}

func TestSendLogData(t *testing.T) {
	// Start an OTLP-compatible receiver.
	ln, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)
	rcv := otlpLogsReceiverOnGRPCServer(ln)
	// Also closes the connection.
	defer rcv.srv.GracefulStop()

	// Start an OTLP exporter and point to the receiver.
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.GRPCClientSettings = configgrpc.GRPCClientSettings{
		Endpoint: ln.Addr().String(),
		TLSSetting: configtls.TLSClientSetting{
			Insecure: true,
		},
	}
	set := exportertest.NewNopCreateSettings()
	set.BuildInfo.Description = "Collector"
	set.BuildInfo.Version = "1.2.3test"
	exp, err := factory.CreateLogsExporter(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)
	defer func() {
		assert.NoError(t, exp.Shutdown(context.Background()))
	}()

	host := componenttest.NewNopHost()

	assert.NoError(t, exp.Start(context.Background(), host))

	// Ensure that initially there is no data in the receiver.
	assert.EqualValues(t, 0, rcv.requestCount.Load())

	// Send empty request.
	ld := plog.NewLogs()
	assert.NoError(t, exp.ConsumeLogs(context.Background(), ld))

	// Wait until it is received.
	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() > 0
	}, 10*time.Second, 5*time.Millisecond)

	// Ensure it was received empty.
	assert.EqualValues(t, 0, rcv.totalItems.Load())

	// A request with 2 log entries.
	ld = testdata.GenerateLogs(2)

	err = exp.ConsumeLogs(context.Background(), ld)
	assert.NoError(t, err)

	// Wait until it is received.
	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() > 1
	}, 10*time.Second, 5*time.Millisecond)

	// Verify received logs.
	assert.EqualValues(t, 2, rcv.requestCount.Load())
	assert.EqualValues(t, 2, rcv.totalItems.Load())
	assert.EqualValues(t, ld, rcv.getLastRequest())

	md := rcv.getMetadata()
	require.Equal(t, len(md.Get("User-Agent")), 1)
	require.Contains(t, md.Get("User-Agent")[0], "Collector/1.2.3test")
}

// TestSendArrowTracesNotSupported tests a successful OTLP export w/
// and without Arrow, w/ WaitForReady and without.
func TestSendArrowTracesNotSupported(t *testing.T) {
	for _, mixed := range []bool{true, false} {
		for _, waitForReady := range []bool{true, false} {
			for _, available := range []bool{true, false} {
				t.Run(fmt.Sprintf("mixed=%v waitForReady=%v available=%v", mixed, waitForReady, available),
					func(t *testing.T) { testSendArrowTraces(t, mixed, waitForReady, available) })
			}
		}
	}
}

func testSendArrowTraces(t *testing.T, mixedSignals, clientWaitForReady, streamServiceAvailable bool) {
	// Start an OTLP-compatible receiver.
	ln, err := net.Listen("tcp", "127.0.0.1:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)

	// Start an OTLP exporter and point to the receiver.
	factory := NewFactory()
	authID := component.NewID("testauth")
	expectedHeader := []string{"arrow-ftw"}
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.GRPCClientSettings = configgrpc.GRPCClientSettings{
		Endpoint: ln.Addr().String(),
		TLSSetting: configtls.TLSClientSetting{
			Insecure: true,
		},
		WaitForReady: clientWaitForReady,
		Headers: map[string]configopaque.String{
			"header": configopaque.String(expectedHeader[0]),
		},
		Auth: &configauth.Authentication{
			AuthenticatorID: authID,
		},
	}
	// Arrow client is enabled, but the server doesn't support it.
	cfg.Arrow = ArrowSettings{
		NumStreams:         1,
		EnableMixedSignals: mixedSignals,
	}

	set := exportertest.NewNopCreateSettings()
	set.TelemetrySettings.Logger = zaptest.NewLogger(t)
	exp, err := factory.CreateTracesExporter(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)

	defer func() {
		assert.NoError(t, exp.Shutdown(context.Background()))
	}()

	type isUserCall struct{}

	host := newHostWithExtensions(
		map[component.ID]component.Component{
			authID: newTestAuthExtension(t, func(ctx context.Context) map[string]string {
				if ctx.Value(isUserCall{}) == nil {
					return nil
				}
				return map[string]string{
					"callerid": "arrow",
				}
			}),
		},
	)
	assert.NoError(t, exp.Start(context.Background(), host))

	rcv, _ := otlpTracesReceiverOnGRPCServer(ln, false)
	if streamServiceAvailable {
		rcv.startStreamMockArrowTraces(t, mixedSignals, okStatusFor)
	}

	// Delay the server start, slightly.
	go func() {
		time.Sleep(100 * time.Millisecond)
		rcv.start()
	}()

	// Send two trace items.
	td := testdata.GenerateTraces(2)

	// Set the context key indicating this is per-request state,
	// so the auth extension returns data.
	err = exp.ConsumeTraces(context.WithValue(context.Background(), isUserCall{}, true), td)
	assert.NoError(t, err)

	// Wait until it is received.
	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() > 0
	}, 10*time.Second, 5*time.Millisecond)

	// Verify two items, one request received.
	assert.EqualValues(t, int32(2), rcv.totalItems.Load())
	assert.EqualValues(t, int32(1), rcv.requestCount.Load())
	assert.EqualValues(t, td, rcv.getLastRequest())

	// Expect the correct metadata, with or without arrow.
	md := rcv.getMetadata()
	require.EqualValues(t, []string{"arrow"}, md.Get("callerid"))
	require.EqualValues(t, expectedHeader, md.Get("header"))
}

func okStatusFor(id string) *arrowpb.StatusMessage {
	return &arrowpb.StatusMessage{
		BatchId:    id,
		StatusCode: arrowpb.StatusCode_OK,
	}
}

func failedStatusFor(id string) *arrowpb.StatusMessage {
	return &arrowpb.StatusMessage{
		BatchId:      id,
		StatusCode:   arrowpb.StatusCode_ERROR,
		ErrorCode:    arrowpb.ErrorCode_INVALID_ARGUMENT,
		ErrorMessage: "test failed",
	}
}

type anyStreamServer interface {
	Send(*arrowpb.BatchStatus) error
	Recv() (*arrowpb.BatchArrowRecords, error)
	grpc.ServerStream
}

func (r *mockTracesReceiver) startStreamMockArrowTraces(t *testing.T, mixedSignals bool, statusFor func(string) *arrowpb.StatusMessage) {
	ctrl := gomock.NewController(t)

	doer := func(server anyStreamServer) error {
		consumer := arrowRecord.NewConsumer()
		var hdrs []hpack.HeaderField
		hdrsDecoder := hpack.NewDecoder(4096, func(hdr hpack.HeaderField) {
			hdrs = append(hdrs, hdr)
		})
		for {
			records, err := server.Recv()
			if status, ok := status.FromError(err); ok && status.Code() == codes.Canceled {
				break
			}
			require.NoError(t, err)
			got, err := consumer.TracesFrom(records)
			require.NoError(t, err)

			// Reset and parse headers
			hdrs = nil
			_, err = hdrsDecoder.Write(records.Headers)
			require.NoError(t, err)
			md, ok := metadata.FromIncomingContext(server.Context())
			require.True(t, ok)

			for _, hf := range hdrs {
				md[hf.Name] = append(md[hf.Name], hf.Value)
			}

			// Place the metadata into the context, where
			// the test framework (independent of Arrow)
			// receives it.
			ctx := metadata.NewIncomingContext(context.Background(), md)

			for _, traces := range got {
				_, err := r.Export(ctx, ptraceotlp.NewExportRequestFromTraces(traces))
				require.NoError(t, err)
			}
			require.NoError(t, server.Send(&arrowpb.BatchStatus{
				Statuses: []*arrowpb.StatusMessage{
					statusFor(records.BatchId),
				},
			}))
		}
		return nil
	}

	if mixedSignals {
		type mixedBinding struct {
			arrowpb.UnsafeArrowStreamServiceServer
			*arrowpbMock.MockArrowStreamServiceServer
		}
		svc := arrowpbMock.NewMockArrowStreamServiceServer(ctrl)

		arrowpb.RegisterArrowStreamServiceServer(r.srv, mixedBinding{
			MockArrowStreamServiceServer: svc,
		})
		svc.EXPECT().ArrowStream(gomock.Any()).Times(1).DoAndReturn(doer)
		return
	}
	type singleBinding struct {
		arrowpb.UnsafeArrowTracesServiceServer
		*arrowpbMock.MockArrowTracesServiceServer
	}
	svc := arrowpbMock.NewMockArrowTracesServiceServer(ctrl)

	arrowpb.RegisterArrowTracesServiceServer(r.srv, singleBinding{
		MockArrowTracesServiceServer: svc,
	})
	svc.EXPECT().ArrowTraces(gomock.Any()).Times(1).DoAndReturn(doer)

}

func TestSendArrowFailedTraces(t *testing.T) {
	// Start an OTLP-compatible receiver.
	ln, err := net.Listen("tcp", "127.0.0.1:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)

	// Start an OTLP exporter and point to the receiver.
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.GRPCClientSettings = configgrpc.GRPCClientSettings{
		Endpoint: ln.Addr().String(),
		TLSSetting: configtls.TLSClientSetting{
			Insecure: true,
		},
		WaitForReady: true,
	}
	// Arrow client is enabled, but the server doesn't support it.
	cfg.Arrow = ArrowSettings{
		NumStreams:         1,
		EnableMixedSignals: true,
	}
	cfg.QueueSettings.Enabled = false

	set := exportertest.NewNopCreateSettings()
	set.TelemetrySettings.Logger = zaptest.NewLogger(t)
	exp, err := factory.CreateTracesExporter(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)

	defer func() {
		assert.NoError(t, exp.Shutdown(context.Background()))
	}()

	host := componenttest.NewNopHost()
	assert.NoError(t, exp.Start(context.Background(), host))

	rcv, _ := otlpTracesReceiverOnGRPCServer(ln, false)
	rcv.startStreamMockArrowTraces(t, true, failedStatusFor)

	// Delay the server start, slightly.
	go func() {
		time.Sleep(100 * time.Millisecond)
		rcv.start()
	}()

	// Send two trace items.
	td := testdata.GenerateTraces(2)
	err = exp.ConsumeTraces(context.Background(), td)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test failed")

	// Wait until it is received.
	assert.Eventually(t, func() bool {
		return rcv.requestCount.Load() > 0
	}, 10*time.Second, 5*time.Millisecond)

	// Verify two items, one request received.
	assert.EqualValues(t, int32(2), rcv.totalItems.Load())
	assert.EqualValues(t, int32(1), rcv.requestCount.Load())
	assert.EqualValues(t, td, rcv.getLastRequest())
}

func TestUserDialOptions(t *testing.T) {
	// Start an OTLP-compatible receiver.
	ln, err := net.Listen("tcp", "127.0.0.1:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)

	// Start an OTLP exporter and point to the receiver.
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.GRPCClientSettings = configgrpc.GRPCClientSettings{
		Endpoint: ln.Addr().String(),
		TLSSetting: configtls.TLSClientSetting{
			Insecure: true,
		},
		WaitForReady: true,
	}
	cfg.Arrow.Disabled = true
	cfg.QueueSettings.Enabled = false

	const testAgent = "test-user-agent (release=:+1:)"

	// This overrides the default provided in otlp.go
	cfg.UserDialOptions = []grpc.DialOption{
		grpc.WithUserAgent(testAgent),
	}

	set := exportertest.NewNopCreateSettings()
	set.TelemetrySettings.Logger = zaptest.NewLogger(t)
	exp, err := factory.CreateTracesExporter(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exp)

	defer func() {
		assert.NoError(t, exp.Shutdown(context.Background()))
	}()

	host := componenttest.NewNopHost()
	assert.NoError(t, exp.Start(context.Background(), host))

	td := testdata.GenerateTraces(2)

	rcv, _ := otlpTracesReceiverOnGRPCServer(ln, false)
	rcv.start()
	defer rcv.srv.GracefulStop()

	err = exp.ConsumeTraces(context.Background(), td)
	assert.NoError(t, err)

	require.Equal(t, len(rcv.getMetadata().Get("User-Agent")), 1)
	require.Contains(t, rcv.getMetadata().Get("User-Agent")[0], testAgent)
}
