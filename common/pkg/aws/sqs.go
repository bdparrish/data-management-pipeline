package aws

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsSdkConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/bparrish/data-management-pipeline/common/pkg/common"
)

type SqsClient interface {
	CreateQueue(queueName string) (*sqs.CreateQueueOutput, error)
	GetQueueUrl(queueName string) (*sqs.GetQueueUrlOutput, error)
	DeleteQueue(queueUrl *string) (*sqs.DeleteQueueOutput, error)
	GetMessages(attributeNames []string, queueURL *string, maxMessages int32, timeout int32) (*sqs.ReceiveMessageOutput, error)
	SendMessage(delay int32, attributes map[string]types.MessageAttributeValue, body string, queueUrl *string) (*sqs.SendMessageOutput, error)
	RemoveMessage(queueURL *string, messageHandle *string) (*sqs.DeleteMessageOutput, error)
}

type Sqs struct {
	Client *sqs.Client
}

func NewSqs() (SqsClient, error) {
	logger, err := common.GetLogger()
	if err != nil {
		log.Println("failed to get logger")
		return Sqs{}, err
	}

	cfg, err := awsSdkConfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		logger.Error(fmt.Sprintf("cannot load the AWS configs: %s", err))
		return Sqs{}, err
	}

	return &Sqs{Client: sqs.NewFromConfig(cfg)}, nil
}

// CreateQueue creates a new Amazon SQS queue.
func (client Sqs) CreateQueue(queueName string) (*sqs.CreateQueueOutput, error) {
	input := &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	}

	return client.Client.CreateQueue(context.TODO(), input)
}

// GetQueueUrl gets the URL of an Amazon SQS queue.
func (client Sqs) GetQueueUrl(queueName string) (*sqs.GetQueueUrlOutput, error) {
	qUInput := &sqs.GetQueueUrlInput{
		QueueName: aws.String(queueName),
	}

	return client.Client.GetQueueUrl(context.TODO(), qUInput)
}

// DeleteQueue deletes an Amazon SQS queue.
func (client Sqs) DeleteQueue(queueUrl *string) (*sqs.DeleteQueueOutput, error) {
	input := &sqs.DeleteQueueInput{
		QueueUrl: queueUrl,
	}

	return client.Client.DeleteQueue(context.TODO(), input)
}

// GetMessages gets the most recent message from an Amazon SQS queue.
func (client Sqs) GetMessages(attributeNames []string, queueURL *string, maxMessages int32, timeout int32) (*sqs.ReceiveMessageOutput, error) {
	input := &sqs.ReceiveMessageInput{
		MessageAttributeNames: attributeNames,
		QueueUrl:              queueURL,
		MaxNumberOfMessages:   maxMessages,
		VisibilityTimeout:     timeout,
	}

	return client.Client.ReceiveMessage(context.TODO(), input)
}

// SendMessage sends a message to an Amazon SQS queue.
func (client Sqs) SendMessage(delay int32, attributes map[string]types.MessageAttributeValue, body string, queueUrl *string) (*sqs.SendMessageOutput, error) {
	input := &sqs.SendMessageInput{
		DelaySeconds:      delay,
		MessageAttributes: attributes,
		MessageBody:       aws.String(body),
		QueueUrl:          queueUrl,
	}

	return client.Client.SendMessage(context.Background(), input)
}

// RemoveMessage deletes a message from an Amazon SQS queue.
func (client Sqs) RemoveMessage(queueURL *string, messageHandle *string) (*sqs.DeleteMessageOutput, error) {
	input := &sqs.DeleteMessageInput{
		QueueUrl:      queueURL,
		ReceiptHandle: messageHandle,
	}

	return client.Client.DeleteMessage(context.TODO(), input)
}
