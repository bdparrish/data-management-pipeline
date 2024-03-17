cwd:=$(shell pwd)
YT:=\033[0;33m
NC:=\033[0m

all: clean lint test tidy

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(firstword $(MAKEFILE_LIST)) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

install:
	brew install mockery golangci-lint protobuf protobuf-gen-go protoc-gen-go-grpc bufbuild/buf/buf
	go install github.com/favadi/protoc-go-inject-tag@latest
	go get github.com/bufbuild/protovalidate-go

generate-proto: ## Build protobuf models
	@echo "$(YT)Building protobuf ...$(NC)"
	buf generate

generate-mocks: ## Build mocks
	@echo "$(YT)Building mocks ...$(NC)"
	mockery --dir ./pkg/aws --name SqsClient --output ./test/mocks --outpkg mocks --filename sqs_mock.go --structname MockSqsClient
	mockery --dir ./pkg/aws --name S3Client --output ./test/mocks --outpkg mocks --filename s3_mock.go --structname MockS3Client
	mockery --dir ./pkg/aws --name StsClient --output ./test/mocks --outpkg mocks --filename sts_mock.go --structname MockStsClient
	mockery --dir ./pkg/repository --name DatasetRepository --output ./test/mocks --outpkg mocks --filename dataset_repository_mock.go --structname MockDatasetRepository
	mockery --dir ./pkg/repository --name GroupRepository --output ./test/mocks --outpkg mocks --filename group_repository_mock.go --structname MockGroupRepository
	mockery --dir ./pkg/repository --name ObjectRepository --output ./test/mocks --outpkg mocks --filename object_repository_mock.go --structname MockObjectRepository
	mockery --dir ./pkg/repository --name SubscriptionRepository --output ./test/mocks --outpkg mocks --filename subscription_repository_mock.go --structname MockSubscriptionRepository
	mockery --dir ./pkg/repository --name UserRepository --output ./test/mocks --outpkg mocks --filename user_repository_mock.go --structname MockUserRepository
	mockery --dir ./pkg/common --name Emailer --output ./test/mocks --outpkg mocks --filename emailer_mock.go --structname MockEmailer
	mockery --dir ./pkg/common --name Tracer --output ./test/mocks --outpkg mocks --filename tracer_mock.go --structname MockTracer
	mockery --dir ./pkg/common/service --name RoleChecker --output ./test/mocks --outpkg mocks --filename role_checker.go --structname MockRoleChecker
	mockery --dir ./pkg/common/service/clients --name GroupsClient --output ./test/mocks --outpkg mocks --filename groups_client_mock.go --structname MockGroupsClient
	mockery --dir ./pkg/common/service/clients --name DatasetsClient --output ./test/mocks --outpkg mocks --filename datasets_client_mock.go --structname MockDatasetsClient
	mockery --dir ./pkg/common/service/clients --name SecurityClient --output ./test/mocks --outpkg mocks --filename security_client_mock.go --structname MockSecurityClient
	mockery --dir ./pkg/common/service/clients --name SubscriptionsClient --output ./test/mocks --outpkg mocks --filename subscriptions_client_mock.go --structname MockSubscriptionsClient
	mockery --dir ./pkg/common/service/clients --name UsersClient --output ./test/mocks --outpkg mocks --filename users_client_mock.go --structname MockUsersClient

generate-all: generate-proto generate-mocks ## Build protobuf models & mocks

clean: ## clean
	@echo "$(YT)Running clean ...$(NC)"
	@go clean -cache -testcache
	@rm -rf ./target

tidy: ## Tidy go mod
	@echo -e "$(YT)Tidying $(C_NAME) ...$(NC)"
	rm -f go.sum
	go mod tidy

lint: clean tidy generate-all ## Run linter
	@echo "$(YT)Linting files ...$(NC)"
	golangci-lint run -v --timeout 10m

test: test-unit test-int ## Run all tests locally

test-unit: generate-all ## Run unit tests locally
	@echo "$(YT)Running unit tests (local) ...$(NC)"
	@mkdir -p ./target/coverage
	@cat ./target/coverage/coverage.out.tmp | grep -v ".pb.go" | grep -v "service/endpoint/" > ./target/coverage/coverage.out
	go tool cover -html=./target/coverage/coverage.out -o ./target/coverage/coverage.html

test-int: generate-all ## Run integration tests - this will start LocalStack, await healthy LocalStack container, run tests, and clean up docker containers.
	$(eval STACK_NAME:=common-test-int)
	@echo "$(YT)Starting compose stack '${STACK_NAME}' for integration tests ...$(NC)"
	@docker compose -p ${STACK_NAME} -f ./test/deployments/docker-compose.yml down --remove-orphans -v
	@docker compose -p ${STACK_NAME} -f ./test/deployments/docker-compose.yml up --exit-code-from integration-test-service --remove-orphans
	@docker compose -p ${STACK_NAME} -f ./test/deployments/docker-compose.yml down --remove-orphans -v
	@echo "$(YT)Tests Complete.$(NC)"
