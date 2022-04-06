module bench

go 1.16

require (
	github.com/LSparkzwz/franz-go v1.0.0
	github.com/LSparkzwz/franz-go/plugin/kprom v0.3.0
	github.com/twmb/tlscfg v1.2.0
)

replace (
	github.com/LSparkzwz/franz-go => ../..
	github.com/LSparkzwz/franz-go/pkg/kmsg => ../../pkg/kmsg
	github.com/LSparkzwz/franz-go/plugin/kprom => ../../plugin/kprom
)
