package aws

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/bparrish/data-management-pipeline/common/pkg/common"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestSqsClient_CreateQueue(t *testing.T) {
	sqsClient, _ := NewSqs()
	testQueueName := "new-test-queue"

	result, err := sqsClient.CreateQueue(testQueueName)
	if err != nil {
		t.Errorf("New queue not created: %v", err)
		return
	}

	assert.NotNil(t, result)
	assert.Regexp(t,
		fmt.Sprintf("^http://sqs\\..*?\\.localhost.*?:4566/000000000000/%s$", testQueueName),
		*result.QueueUrl,
		"Wrong queue created",
	)
}

func TestSqsClient_GetQueueUrl(t *testing.T) {
	conf := common.GetConfig()
	sqsClient, _ := NewSqs()

	result, err := sqsClient.GetQueueUrl(conf.AwsSqsIngestQueue)
	if err != nil {
		t.Errorf("Got an error getting the queue URL: %v", err)
		return
	}

	assert.NotNil(t, result)
	assert.Regexp(t,
		fmt.Sprintf("^http://sqs\\..*?\\.localhost.*?:4566/000000000000/%s$", conf.AwsSqsIngestQueue),
		*result.QueueUrl,
		"Wrong queue created",
	)
}

func TestSqsClient_SendMessage(t *testing.T) {
	conf := common.GetConfig()
	sqsClient, _ := NewSqs()

	// Get URL of queue
	result, err := sqsClient.GetQueueUrl(conf.AwsSqsIngestQueue)
	if err != nil {
		t.Errorf("Got an error getting the queue URL: %v", err)
		return
	}

	resp, err := sqsClient.SendMessage(10, map[string]types.MessageAttributeValue{
		"Title": {
			DataType:    aws.String("String"),
			StringValue: aws.String("The Whistler"),
		},
		"Author": {
			DataType:    aws.String("String"),
			StringValue: aws.String("John Grisham"),
		},
		"WeeksOn": {
			DataType:    aws.String("Number"),
			StringValue: aws.String("6"),
		},
	}, "Information about current NY Times fiction bestseller for week of 12/11/2016.", result.QueueUrl)
	if err != nil {
		t.Errorf("Got an error sending the message: %v", err)
		return
	}

	_, err = uuid.Parse(*resp.MessageId)
	assert.NoError(t, err, "Received bad MessageId when sending SQS message")
}

func TestSqsClient_GetMessages(t *testing.T) {
	conf := common.GetConfig()
	sqsClient, _ := NewSqs()

	// Get URL of queue
	result, err := sqsClient.GetQueueUrl(conf.AwsSqsIngestQueue)
	if err != nil {
		t.Errorf("Got an error getting the queue URL: %v", err)
		return
	}

	_, err = sqsClient.SendMessage(10, map[string]types.MessageAttributeValue{
		"Title": {
			DataType:    aws.String("String"),
			StringValue: aws.String("The Whistler"),
		},
		"Author": {
			DataType:    aws.String("String"),
			StringValue: aws.String("John Grisham"),
		},
		"WeeksOn": {
			DataType:    aws.String("Number"),
			StringValue: aws.String("6"),
		},
	}, "Information about current NY Times fiction bestseller for week of 12/11/2016.", result.QueueUrl)
	if err != nil {
		t.Errorf("Got an error sending the message: %v", err)
		return
	}

	msgResult, err := sqsClient.GetMessages([]string{string(types.QueueAttributeNameAll)}, result.QueueUrl, 1, 60)
	if err != nil {
		t.Errorf("Got an error receiving messages: %v", err)
		return
	}

	assert.NotNil(t, msgResult.Messages, "No messages found")
	assert.Equal(t, len(msgResult.Messages), 1)
}

func TestSqsClient_RemoveMessage(t *testing.T) {
	conf := common.GetConfig()
	sqsClient, _ := NewSqs()

	// Get URL of queue
	result, err := sqsClient.GetQueueUrl(conf.AwsSqsIngestQueue)
	if err != nil {
		t.Errorf("Got an error getting the queue URL: %v", err)
		return
	}

	_, err = sqsClient.SendMessage(10, map[string]types.MessageAttributeValue{
		"Title": {
			DataType:    aws.String("String"),
			StringValue: aws.String("The Whistler"),
		},
		"Author": {
			DataType:    aws.String("String"),
			StringValue: aws.String("John Grisham"),
		},
		"WeeksOn": {
			DataType:    aws.String("Number"),
			StringValue: aws.String("6"),
		},
	}, "Information about current NY Times fiction bestseller for week of 12/11/2016.", result.QueueUrl)
	if err != nil {
		t.Errorf("Got an error sending the message: %v", err)
		return
	}

	msgResult, err := sqsClient.GetMessages([]string{string(types.QueueAttributeNameAll)}, result.QueueUrl, 1, 60)
	if err != nil {
		t.Errorf("Got an error receiving messages: %v", err)
		return
	}

	_, err = sqsClient.RemoveMessage(result.QueueUrl, msgResult.Messages[0].ReceiptHandle)
	assert.NoError(t, err, fmt.Sprintf("Got an error deleting the message: %v", err))
}

func TestSqsClient_DeleteQueue(t *testing.T) {
	conf := common.GetConfig()
	sqsClient, _ := NewSqs()

	qUrl, err := sqsClient.GetQueueUrl(conf.AwsSqsIngestQueue)
	if err != nil {
		t.Errorf("Got an error getting the queue URL: %v", err)
		return
	}

	_, err = sqsClient.DeleteQueue(aws.String(*qUrl.QueueUrl))
	assert.NoError(t, err, fmt.Sprintf("Got an error deleting the queue: %v", err))
}
