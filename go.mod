module github.com/f5/otel-arrow-adapter

go 1.19

require (
	github.com/HdrHistogram/hdrhistogram-go v1.1.2
	github.com/apache/arrow/go/v12 v12.0.0-20230404000714-f02d35119ae6
	github.com/axiomhq/hyperloglog v0.0.0-20230201085229-3ddf4bad03dc
	github.com/brianvoe/gofakeit/v6 v6.17.0
	github.com/dustin/go-humanize v1.0.1
	github.com/fxamacker/cbor/v2 v2.4.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang/mock v1.6.0
	github.com/klauspost/compress v1.16.5
	github.com/olekukonko/tablewriter v0.0.5
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter v0.77.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension v0.77.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension v0.77.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filereceiver v0.77.0
	github.com/pierrec/lz4 v2.0.5+incompatible
	github.com/stretchr/testify v1.8.2
	go.opentelemetry.io/collector v0.78.1
	go.opentelemetry.io/collector/component v0.78.1
	go.opentelemetry.io/collector/confmap v0.78.1
	go.opentelemetry.io/collector/consumer v0.78.1
	go.opentelemetry.io/collector/exporter v0.78.1
	go.opentelemetry.io/collector/exporter/loggingexporter v0.78.1
	go.opentelemetry.io/collector/exporter/otlphttpexporter v0.78.1
	go.opentelemetry.io/collector/extension/ballastextension v0.78.1
	go.opentelemetry.io/collector/extension/zpagesextension v0.78.1
	go.opentelemetry.io/collector/pdata v1.0.0-rcv0012
	go.opentelemetry.io/collector/processor/batchprocessor v0.78.1
	go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.78.1
	go.opentelemetry.io/collector/receiver v0.78.1
	go.opentelemetry.io/collector/semconv v0.78.1
	go.opentelemetry.io/otel v1.15.1
	go.opentelemetry.io/otel/metric v0.38.1
	go.opentelemetry.io/otel/sdk v1.15.1
	go.opentelemetry.io/otel/sdk/metric v0.38.1
	go.opentelemetry.io/proto/otlp v0.19.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.24.0
	golang.org/x/exp v0.0.0-20230321023759-10a507213a29
	golang.org/x/net v0.10.0
	golang.org/x/sys v0.8.0
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1
	google.golang.org/grpc v1.55.0
	google.golang.org/protobuf v1.30.0
)

require (
	contrib.go.opencensus.io/exporter/prometheus v0.4.2 // indirect
	github.com/GehirnInc/crypt v0.0.0-20200316065508-bb7000b8a962 // indirect
	github.com/andybalholm/brotli v1.0.4 // indirect
	github.com/apache/thrift v0.16.0 // indirect
	github.com/benbjohnson/clock v1.3.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgryski/go-metro v0.0.0-20180109044635-280f6062b5bc // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/goccy/go-json v0.9.11 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/flatbuffers v2.0.8+incompatible // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.7.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/asmfmt v1.3.2 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/minio/asm2plan9s v0.0.0-20200509001527-cdd76441f9d8 // indirect
	github.com/minio/c2goasm v0.0.0-20190812172519-36a3d3bbc4f3 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mostynb/go-grpc-compression v1.1.18 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent v0.77.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.17 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/prometheus/client_golang v1.15.1 // indirect
	github.com/prometheus/client_model v0.4.0 // indirect
	github.com/prometheus/common v0.43.0 // indirect
	github.com/prometheus/procfs v0.9.0 // indirect
	github.com/prometheus/statsd_exporter v0.22.7 // indirect
	github.com/rs/cors v1.9.0 // indirect
	github.com/shirou/gopsutil/v3 v3.23.4 // indirect
	github.com/shoenig/go-m1cpu v0.1.5 // indirect
	github.com/spf13/cobra v1.7.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/tg123/go-htpasswd v1.2.1 // indirect
	github.com/tklauser/go-sysconf v0.3.11 // indirect
	github.com/tklauser/numcpus v0.6.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	github.com/zeebo/xxh3 v1.0.2 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.0.0-rcv0012 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.41.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.41.1 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.16.1 // indirect
	go.opentelemetry.io/contrib/zpages v0.41.1 // indirect
	go.opentelemetry.io/otel/bridge/opencensus v0.38.1 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.38.1 // indirect
	go.opentelemetry.io/otel/trace v1.15.1 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	golang.org/x/crypto v0.8.0 // indirect
	golang.org/x/mod v0.9.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	golang.org/x/tools v0.7.0 // indirect
	golang.org/x/xerrors v0.0.0-20220609144429-65e65417b02f // indirect
	gonum.org/v1/gonum v0.13.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
