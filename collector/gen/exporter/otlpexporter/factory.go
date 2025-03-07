// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpexporter // import "github.com/f5/otel-arrow-adapter/collector/gen/exporter/otlpexporter"

import (
	"context"
	"runtime"

	arrowpb "github.com/f5/otel-arrow-adapter/api/experimental/arrow/v1"
	"google.golang.org/grpc"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"github.com/f5/otel-arrow-adapter/collector/gen/exporter/otlpexporter/internal/arrow"
)

const (
	// The value of "type" key in configuration.
	typeStr = "otlp"
)

// NewFactory creates a factory for OTLP exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, component.StabilityLevelStable),
		exporter.WithMetrics(createMetricsExporter, component.StabilityLevelStable),
		exporter.WithLogs(createLogsExporter, component.StabilityLevelBeta),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
		RetrySettings:   exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
		GRPCClientSettings: configgrpc.GRPCClientSettings{
			Headers: map[string]configopaque.String{},
			// Default to gzip compression
			Compression: configcompression.Gzip,
			// We almost read 0 bytes, so no need to tune ReadBufferSize.
			WriteBufferSize: 512 * 1024,
		},
		Arrow: ArrowSettings{
			NumStreams: runtime.NumCPU(),
		},
	}
}

func (oce *baseExporter) helperOptions() []exporterhelper.Option {
	return []exporterhelper.Option{
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithTimeout(oce.config.TimeoutSettings),
		exporterhelper.WithRetry(oce.config.RetrySettings),
		exporterhelper.WithQueue(oce.config.QueueSettings),
		exporterhelper.WithStart(oce.start),
		exporterhelper.WithShutdown(oce.shutdown),
	}
}

func createArrowTracesStream(cfg *Config, conn *grpc.ClientConn) func(ctx context.Context, opts ...grpc.CallOption) (arrow.AnyStreamClient, error) {
	if cfg.Arrow.EnableMixedSignals {
		return arrow.MakeAnyStreamClient(arrowpb.NewArrowStreamServiceClient(conn).ArrowStream)
	}
	return arrow.MakeAnyStreamClient(arrowpb.NewArrowTracesServiceClient(conn).ArrowTraces)
}

func createTracesExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Traces, error) {
	oce, err := newExporter(cfg, set, createArrowTracesStream)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewTracesExporter(ctx, oce.settings, oce.config,
		oce.pushTraces,
		oce.helperOptions()...,
	)
}

func createArrowMetricsStream(cfg *Config, conn *grpc.ClientConn) func(ctx context.Context, opts ...grpc.CallOption) (arrow.AnyStreamClient, error) {
	if cfg.Arrow.EnableMixedSignals {
		return arrow.MakeAnyStreamClient(arrowpb.NewArrowStreamServiceClient(conn).ArrowStream)
	}
	return arrow.MakeAnyStreamClient(arrowpb.NewArrowMetricsServiceClient(conn).ArrowMetrics)
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Metrics, error) {
	oce, err := newExporter(cfg, set, createArrowMetricsStream)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewMetricsExporter(ctx, oce.settings, oce.config,
		oce.pushMetrics,
		oce.helperOptions()...,
	)
}

func createArrowLogsStream(cfg *Config, conn *grpc.ClientConn) func(ctx context.Context, opts ...grpc.CallOption) (arrow.AnyStreamClient, error) {
	if cfg.Arrow.EnableMixedSignals {
		return arrow.MakeAnyStreamClient(arrowpb.NewArrowStreamServiceClient(conn).ArrowStream)
	}
	return arrow.MakeAnyStreamClient(arrowpb.NewArrowLogsServiceClient(conn).ArrowLogs)
}

func createLogsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Logs, error) {
	oce, err := newExporter(cfg, set, createArrowLogsStream)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewLogsExporter(ctx, oce.settings, oce.config,
		oce.pushLogs,
		oce.helperOptions()...,
	)
}
