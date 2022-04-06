module dropin_prometheus

go 1.16

require (
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475
	github.com/LSparkzwz/franz-go v0.8.3
	github.com/LSparkzwz/franz-go/plugin/kgmetrics v0.1.0
)

replace github.com/LSparkzwz/franz-go => ../../..
