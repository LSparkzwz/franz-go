module aws_msk_iam

go 1.16

require (
	github.com/aws/aws-sdk-go v1.40.42
	github.com/LSparkzwz/franz-go v0.8.3
	github.com/LSparkzwz/franz-go/pkg/kmsg v1.0.0
)

replace github.com/LSparkzwz/franz-go => ../../..
