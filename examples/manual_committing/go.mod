module manual_committing

go 1.16

require (
	github.com/LSparkzwz/franz-go v1.2.3-0.20211104052441-7952375c09c0
	github.com/LSparkzwz/franz-go/pkg/kadm v0.0.0-20211016003631-fbf9239e2698
)

replace github.com/LSparkzwz/franz-go => ../..

replace github.com/LSparkzwz/franz-go/pkg/kadm => ../../pkg/kadm
