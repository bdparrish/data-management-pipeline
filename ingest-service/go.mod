module github.com/bparrish/data-management-pipeline/ingest-service

go 1.22

require (
	buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.32.0-20231115204500-e097f827e652.1
	github.com/DATA-DOG/go-sqlmock v1.5.0
	github.com/aws/aws-sdk-go-v2 v1.24.0
	github.com/aws/aws-sdk-go-v2/config v1.26.2
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.15.9
	github.com/aws/aws-sdk-go-v2/service/s3 v1.47.7
	github.com/aws/aws-sdk-go-v2/service/sqs v1.29.6
	github.com/aws/aws-sdk-go-v2/service/sts v1.26.6
	github.com/aws/smithy-go v1.19.0
	github.com/bufbuild/protovalidate-go v0.4.3
	github.com/docker/go-units v0.5.0
	github.com/elastic/go-elasticsearch/v8 v8.9.0
	github.com/google/uuid v1.5.0
	github.com/samber/lo v1.39.0
	github.com/spf13/viper v1.18.2
	github.com/stretchr/testify v1.8.4
	golang.org/x/exp v0.0.0-20231226003508-02704c960a9b
	google.golang.org/protobuf v1.32.0
	gorm.io/driver/postgres v1.5.4
	gorm.io/gorm v1.25.5
)
