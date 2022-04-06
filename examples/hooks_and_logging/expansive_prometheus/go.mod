module prometheus_hooks

go 1.16

require (
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/common v0.30.0 // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/LSparkzwz/franz-go v0.8.3
	google.golang.org/protobuf v1.27.1 // indirect
)

replace github.com/LSparkzwz/franz-go => ../../..
