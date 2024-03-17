package aws

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bparrish/data-management-pipeline/common/pkg/common"
	"github.com/docker/go-units"
	"github.com/stretchr/testify/assert"
)

func TestS3Client_HeadObject(t *testing.T) {
	s3Client, _ := NewS3()

	conf := common.GetConfig()

	objectKey := "test/something/test.txt"

	_, err := uploadLocalObject(
		s3Client,
		conf.AwsS3IngestBucket,
		objectKey,
		"../../test/files/test.txt",
		map[string]string{fmt.Sprintf("%s-test-key", conf.AwsS3MetadataPrefix): "test value"},
	)
	if err != nil {
		t.Error("failed to upload a test file to the test bucket")
		return
	}

	headObject, err := s3Client.HeadObject(conf.AwsS3IngestBucket, objectKey)
	if err != nil {
		t.Error("failed to get object metadata")
		return
	}

	assert.NotNil(t, headObject.Metadata)
	assert.Equal(t, "test value", headObject.Metadata[fmt.Sprintf("%s-test-key", conf.AwsS3MetadataPrefix)])
}

func TestS3Client_HeadObjectBadBucket(t *testing.T) {
	s3Client, _ := NewS3()

	conf := common.GetConfig()

	objectKey := "test/something"

	_, err := uploadLocalObject(
		s3Client,
		conf.AwsS3IngestBucket,
		objectKey,
		"../../test/files/test.txt",
		map[string]string{fmt.Sprintf("%s-test_key", conf.AwsS3MetadataPrefix): "test value"},
	)
	if err != nil {
		t.Error("failed to upload a test file to the test bucket")
		return
	}

	_, err = s3Client.HeadObject("bad-bucket-metadata", objectKey)
	if err == nil {
		t.Error("successfully retrieved metadata from a bad bucket, but expected it to fail")
		return
	}

	assert.Error(t, err)
}

func TestS3Client_HeadVersionedObject_Success(t *testing.T) {
	s3Client, _ := NewS3()
	conf := common.GetConfig()

	objectKey := "test/something/test.txt"

	putResponse, err := uploadLocalObject(
		s3Client,
		conf.AwsS3IngestBucket,
		objectKey,
		"../../test/files/test.txt",
		map[string]string{fmt.Sprintf("%s-test-key", conf.AwsS3MetadataPrefix): "test value"},
	)
	if err != nil {
		t.Error("failed to upload a test file to the test bucket")
		return
	}

	assert.NotNil(t, putResponse.VersionId)
	assert.NotEmpty(t, putResponse.VersionId)

	headObject, err := s3Client.HeadVersionedObject(conf.AwsS3IngestBucket, objectKey, *putResponse.VersionId)
	if err != nil {
		t.Error("failed to get object metadata")
		return
	}

	assert.NotNil(t, headObject.Metadata)
	assert.Equal(t, "test value", headObject.Metadata[fmt.Sprintf("%s-test-key", conf.AwsS3MetadataPrefix)])
}

func TestS3Client_HeadVersionedObject_Error(t *testing.T) {
	s3Client, _ := NewS3()
	conf := common.GetConfig()

	objectKey := "test/something"

	putResponse, err := uploadLocalObject(
		s3Client,
		conf.AwsS3IngestBucket,
		objectKey,
		"../../test/files/test.txt",
		map[string]string{fmt.Sprintf("%s-test_key", conf.AwsS3MetadataPrefix): "test value"},
	)
	if err != nil {
		t.Error("failed to upload a test file to the test bucket")
		return
	}

	assert.NotNil(t, putResponse.VersionId)
	assert.NotEmpty(t, putResponse.VersionId)

	_, err = s3Client.HeadVersionedObject("bad-bucket-metadata", objectKey, *putResponse.VersionId)

	assert.Error(t, err, "expected an error with non-existent bucket")
}

func TestS3Client_CopyVersionedObject_Success(t *testing.T) {
	s3Client, _ := NewS3()
	conf := common.GetConfig()

	objectKey := "test/something/test.txt"

	putResponse, err := uploadLocalObject(
		s3Client,
		conf.AwsS3IngestBucket,
		objectKey,
		"../../test/files/test.txt",
		map[string]string{"test-key-2": "test value 2"},
	)
	if err != nil {
		t.Error("failed to upload a test file to the test bucket")
		return
	}

	assert.NotNil(t, putResponse.VersionId)
	assert.NotEmpty(t, putResponse.VersionId)

	source := fmt.Sprintf("%s/%s", conf.AwsS3IngestBucket, objectKey)

	origMetadata, err := s3Client.HeadVersionedObject(conf.AwsS3IngestBucket, objectKey, *putResponse.VersionId)
	if err != nil {
		return
	}

	origMetadata.Metadata["test-key"] = "test value"

	_, err = s3Client.CopyVersionedObject(conf.AwsS3RegistryBucket, objectKey, source, *putResponse.VersionId, origMetadata.Metadata)
	if err != nil {
		t.Error("failed to copy object to another bucket")
		return
	}

	metadata, err := s3Client.HeadObject(conf.AwsS3RegistryBucket, objectKey)
	if err != nil {
		return
	}

	assert.Equal(t, 2, len(metadata.Metadata))
	assert.Equal(t, "test value", metadata.Metadata["test-key"])
	assert.Equal(t, "test value 2", metadata.Metadata["test-key-2"])
}

func TestS3Client_CopyVersionedObject_Error(t *testing.T) {
	s3Client, _ := NewS3()
	conf := common.GetConfig()

	objectKey := "test/something"

	putResponse, err := uploadLocalObject(
		s3Client,
		conf.AwsS3IngestBucket,
		objectKey,
		"../../test/files/test.txt", map[string]string{},
	)
	if err != nil {
		t.Error("failed to upload a test file to the test bucket")
		return
	}

	assert.NotNil(t, putResponse.VersionId)
	assert.NotEmpty(t, putResponse.VersionId)

	source := fmt.Sprintf("%s/%s", conf.AwsS3IngestBucket, "test.txt")

	_, err = s3Client.CopyVersionedObject("bad-bucket-copy", objectKey, source, *putResponse.VersionId, map[string]string{})

	assert.Error(t, err)
}

func TestS3Client_CopyVersionedObject_EmptySourceVersion(t *testing.T) {
	s3Client, _ := NewS3()
	conf := common.GetConfig()

	objectKey := "test/something/test.txt"
	source := fmt.Sprintf("%s/%s", conf.AwsS3IngestBucket, objectKey)

	_, err := s3Client.CopyVersionedObject("bad-bucket-copy", objectKey, source, "  ", map[string]string{})
	assert.EqualError(t, err, "'sourceVersion' cannot be empty")
}

func TestS3Client_MultipartCopyObject_VersionedObject(t *testing.T) {
	s3Client, _ := NewS3()
	conf := common.GetConfig()

	objectKey := "test/something/test.txt"

	fi, _ := os.Stat("../../test/files/300MB.txt")
	// get the size
	size := fi.Size()

	putResponse, err := uploadLocalObject(
		s3Client,
		conf.AwsS3IngestBucket,
		objectKey,
		"../../test/files/300MB.txt",
		map[string]string{"test-key-2": "test value 2"},
	)
	if err != nil {
		t.Error("failed to upload a test file to the test bucket")
		return
	}

	assert.NotNil(t, putResponse.VersionId)
	assert.NotEmpty(t, putResponse.VersionId)

	source := fmt.Sprintf("%s/%s", conf.AwsS3IngestBucket, objectKey)

	origMetadata, err := s3Client.HeadVersionedObject(conf.AwsS3IngestBucket, objectKey, *putResponse.VersionId)
	if err != nil {
		return
	}

	origMetadata.Metadata["test-key"] = "test value"

	_, err = s3Client.MultipartCopyVersionedObject(conf.AwsS3RegistryBucket, objectKey, source, *putResponse.VersionId, size, origMetadata.Metadata)
	if err != nil {
		t.Error("failed to copy object to another bucket")
		return
	}

	metadata, err := s3Client.HeadObject(conf.AwsS3RegistryBucket, objectKey)
	if err != nil {
		return
	}

	assert.Equal(t, 2, len(metadata.Metadata))
	assert.Equal(t, "test value", metadata.Metadata["test-key"])
	assert.Equal(t, "test value 2", metadata.Metadata["test-key-2"])
}

func TestS3Client_UploadObject(t *testing.T) {
	s3Client, _ := NewS3()

	conf := common.GetConfig()

	largeBytes := make([]byte, 128*2*units.MiB)
	_, err := rand.Read(largeBytes)
	if err != nil {
		t.Error("Error in generating random payload for S3 upload")
		return
	}

	_, err = s3Client.UploadObject(
		conf.AwsS3IngestBucket,
		"test/something/large",
		bytes.NewReader(largeBytes),
		map[string]string{},
		aws.String("application/octet-stream"),
	)
	if err != nil {
		t.Error("failed to upload a large test file to the test bucket")
		return
	}
}

func TestS3_UploadObject_BadUploadObjectRequest(t *testing.T) {
	s3Client, _ := NewS3()

	largeBytes := make([]byte, 1024*2*units.MiB)
	_, err := rand.Read(largeBytes)
	if err != nil {
		t.Error("Error in generating random payload for S3 upload")
		return
	}

	_, err = s3Client.UploadObject(
		"bad-bucket",
		"test/something/large",
		bytes.NewReader(largeBytes),
		map[string]string{},
		aws.String("application/octet-stream"),
	)
	if err == nil {
		t.Error("successfully found the file, but expected an error")
		return
	}

	if !strings.Contains(err.Error(), "NoSuchBucket") {
		t.Error("wrong error returned; expected `NoSuchBucket`")
		return
	}
}

func TestS3Client_DownloadObject(t *testing.T) {
	s3Client, _ := NewS3()

	conf := common.GetConfig()

	resp, err := s3Client.DownloadObject(conf.AwsS3IngestBucket, "test/something", "something", 3600)
	if err != nil {
		t.Error("failed to get presigned URL")
		return
	}

	if !strings.Contains(resp.URL, "X-Amz-Expires=3600") {
		t.Error("Expiration time was wrong; expected`X-Amz-Expires=3600`")
		return
	}
}

func TestS3Client_ObjectExists(t *testing.T) {
	s3Client, _ := NewS3()

	conf := common.GetConfig()

	_, err := uploadLocalObject(
		s3Client,
		conf.AwsS3IngestBucket,
		"test/something",
		"../../test/files/test.txt",
		map[string]string{},
	)
	if err != nil {
		t.Error("failed to upload a test file to the test bucket")
		return
	}

	exists, err := s3Client.ObjectExists(conf.AwsS3IngestBucket, "test/something")
	if err != nil {
		t.Error("failed to check if the object exists in s3")
		return
	}

	if !exists {
		t.Error("Object doesn't exist like expected")
		return
	}
}

func TestS3Client_ObjectDoesNotExists_Success(t *testing.T) {
	s3Client, _ := NewS3()

	conf := common.GetConfig()

	_, err := uploadLocalObject(
		s3Client,
		conf.AwsS3IngestBucket,
		"test/something",
		"../../test/files/test.txt",
		map[string]string{},
	)
	if err != nil {
		t.Error("failed to upload a test file to the test bucket")
		return
	}

	answer, err := s3Client.ObjectExists(conf.AwsS3IngestBucket, "test/anything")

	assert.NoError(t, err)
	assert.False(t, answer)
}

func TestS3Client_DeleteObjects(t *testing.T) {
	s3Client, _ := NewS3()

	conf := common.GetConfig()

	_, err := uploadLocalObject(
		s3Client,
		conf.AwsS3IngestBucket,
		"test/something",
		"../../test/files/test.txt",
		map[string]string{},
	)
	if err != nil {
		t.Error("failed to upload a test file to the test bucket")
		return
	}

	resp, err := s3Client.DeleteObjects(conf.AwsS3IngestBucket, []string{"test/something"})
	if err != nil {
		t.Error("failed to delete object from the bucket")
		return
	}

	if "test/something" != *resp.Deleted[0].Key {
		t.Error("Deleted the wrong file")
		return
	}
}

func TestS3_DeleteObject_BadDeleteObjectRequest(t *testing.T) {
	s3Client, _ := NewS3()

	largeBytes := make([]byte, 1024*2*units.MiB)
	_, err := rand.Read(largeBytes)
	if err != nil {
		t.Error("Error in generating random payload for S3 upload")
		return
	}

	_, err = s3Client.DeleteObjects("bad-bucket", []string{"test/something"})
	if err == nil {
		t.Error("successfully found the file, but expected an error")
		return
	}

	if !strings.Contains(err.Error(), "NoSuchBucket") {
		t.Error("wrong error returned; expected `NoSuchBucket`")
		return
	}
}

func TestS3Client_DeleteVersionedObject_Success(t *testing.T) {
	s3Client, _ := NewS3()
	conf := common.GetConfig()

	objectKey := "test/something"
	putResponse, err := uploadLocalObject(
		s3Client,
		conf.AwsS3IngestBucket,
		objectKey,
		"../../test/files/test.txt",
		map[string]string{},
	)
	if err != nil {
		t.Error("failed to upload a test file to the test bucket")
		return
	}

	assert.NotNil(t, putResponse.VersionId)
	assert.NotEmpty(t, putResponse.VersionId)

	resp, err := s3Client.DeleteVersionedObject(conf.AwsS3IngestBucket, objectKey, *putResponse.VersionId)

	assert.NoError(t, err)
	assert.Len(t, resp.Deleted, 1)
	assert.Equal(t, objectKey, *resp.Deleted[0].Key)
}

func TestS3Client_DeleteVersionedObject_Error(t *testing.T) {
	s3Client, _ := NewS3()

	_, err := s3Client.DeleteVersionedObject("bad-bucket", "test/something", "non-existent-version")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NoSuchBucket")
}

func uploadLocalObject(s3Client S3, bucketName string, objectKey string, fileName string, metadata map[string]string) (*s3.PutObjectOutput, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}

	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	putObjectInput := &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(objectKey),
		Body:        file,
		Metadata:    metadata,
		ContentType: aws.String("text/plain"),
	}

	return s3Client.Client.PutObject(context.TODO(), putObjectInput)
}
