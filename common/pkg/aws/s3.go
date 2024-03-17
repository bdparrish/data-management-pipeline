package aws

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsSdkConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/bparrish/data-management-pipeline/common/pkg/common"
)

const (
	partSize = 100 * 1024 * 1024
	retries  = 2
)

var wg = sync.WaitGroup{}

type partUploadResult struct {
	completedPart *types.CompletedPart
	err           error
}

type S3Client interface {
	HeadObject(bucketName string, objectKey string) (*s3.HeadObjectOutput, error)
	HeadVersionedObject(bucket, key, version string) (*s3.HeadObjectOutput, error)
	CopyVersionedObject(bucketName string, objectKey string, source string, sourceVersion string, metadata map[string]string) (*s3.CopyObjectOutput, error)
	MultipartCopyVersionedObject(bucketName string, objectKey string, source string, sourceVersion string, fileSize int64, metadata map[string]string) (*s3.CompleteMultipartUploadOutput, error)
	UploadObject(bucketName string, objectKey string, reader io.Reader, metadata map[string]string, contentType *string) (*manager.UploadOutput, error)
	DownloadObject(bucketName string, objectKey string, downloadName string, lifetimeSecs int64) (*v4.PresignedHTTPRequest, error)
	ObjectExists(bucketName string, objectKey string) (bool, error)
	DeleteObjects(bucketName string, objectKeys []string) (*s3.DeleteObjectsOutput, error)
	DeleteVersionedObject(bucketName string, objectKey string, objectVersion string) (*s3.DeleteObjectsOutput, error)
}

type S3 struct {
	Client        *s3.Client
	PresignClient *s3.PresignClient
	Logger        common.Logger
}

func NewS3() (S3, error) {
	logger, err := common.GetLogger()
	if err != nil {
		log.Println("failed to get logger")
		return S3{}, err
	}

	cfg, err := awsSdkConfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		logger.Error(fmt.Sprintf("cannot load the AWS configs: %s", err))
		return S3{}, err
	}

	c := s3.NewFromConfig(cfg)
	pc := s3.NewPresignClient(c)

	s3Client := S3{
		Client:        c,
		PresignClient: pc,
		Logger:        logger,
	}

	return s3Client, nil
}

func (client *S3) HeadObject(bucket, key string) (*s3.HeadObjectOutput, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	result, err := client.Client.HeadObject(context.TODO(), input)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (client *S3) HeadVersionedObject(bucket, key, version string) (*s3.HeadObjectOutput, error) {
	input := &s3.HeadObjectInput{
		Bucket:    aws.String(bucket),
		Key:       aws.String(key),
		VersionId: aws.String(version),
	}

	result, err := client.Client.HeadObject(context.TODO(), input)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// CopyVersionedObject copies a versioned object from one S3 bucket to another bucket
func (client *S3) CopyVersionedObject(
	bucketName string,
	objectKey string,
	source string,
	sourceVersion string,
	metadata map[string]string,
) (*s3.CopyObjectOutput, error) {
	if strings.TrimSpace(sourceVersion) == "" {
		return nil, fmt.Errorf("'sourceVersion' cannot be empty")
	}

	source = fmt.Sprintf("%s?versionId=%s", url.PathEscape(source), url.QueryEscape(sourceVersion))

	resp, err := client.Client.CopyObject(context.TODO(), &s3.CopyObjectInput{
		Bucket:            aws.String(bucketName),
		CopySource:        aws.String(source),
		Key:               aws.String(objectKey),
		MetadataDirective: types.MetadataDirectiveReplace,
		Metadata:          metadata,
		ContentType:       aws.String(metadata["content-type"]),
	})
	if err != nil {
		client.Logger.Error(fmt.Sprintf(
			"couldn't copy object to %v:%v from %v?versionId=%s. Error: %v",
			bucketName, objectKey, source, sourceVersion, err,
		))
		return nil, err
	}

	return resp, nil
}

// MultipartCopyVersionedObject copies a versioned object from one S3 bucket to another bucket using a multipart upload
func (client *S3) MultipartCopyVersionedObject(bucketName string, objectKey string, source string, sourceVersion string, fileSize int64, metadata map[string]string) (*s3.CompleteMultipartUploadOutput, error) {
	if strings.TrimSpace(sourceVersion) == "" {
		return nil, fmt.Errorf("'sourceVersion' cannot be empty")
	}

	escapedSource := fmt.Sprintf("%s?versionId=%s", url.PathEscape(source), url.QueryEscape(sourceVersion))

	expiryDate := time.Now().AddDate(0, 0, 1)

	createUploadOuput, err := client.Client.CreateMultipartUpload(context.TODO(), &s3.CreateMultipartUploadInput{
		Bucket:   aws.String(bucketName),
		Key:      aws.String(objectKey),
		Expires:  &expiryDate,
		Metadata: metadata,
	})
	if err != nil {
		client.Logger.Error(fmt.Sprintf(
			"couldn't complete multipart copy object to %v:%v from %v. Error: %v",
			bucketName, objectKey, source, err,
		))
		return nil, err
	}

	ch := make(chan partUploadResult)

	var start, currentSize int
	var remaining = int(fileSize)
	var partNum = 1
	var completedParts []types.CompletedPart
	for start = 0; remaining > 0; start += partSize {
		wg.Add(1)
		if remaining < partSize {
			currentSize = remaining
		} else {
			currentSize = partSize
		}
		go client.uploadMultipartToS3(createUploadOuput, escapedSource, start, start+currentSize, partNum, ch, &wg)

		remaining -= currentSize
		client.Logger.Debug(fmt.Sprintf("uploading of part %v started and remaining is %v \n", partNum, remaining))
		partNum++
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for result := range ch {
		if result.err != nil {
			_, err = client.Client.AbortMultipartUpload(context.TODO(), &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(bucketName),
				Key:      aws.String(objectKey),
				UploadId: createUploadOuput.UploadId,
			})
			if err != nil {
				client.Logger.Error(fmt.Sprintf(
					"aborted multipart - couldn't complete multipart copy object to %v:%v from %v. Error: %v",
					bucketName, objectKey, source, err,
				))
				return nil, err
			}
			return nil, result.err
		}
		client.Logger.Debug(fmt.Sprintf("uploading of part %v has been finished \n", *result.completedPart.PartNumber))
		completedParts = append(completedParts, *result.completedPart)
	}

	// Ordering the array based on the PartNumber as each parts could be uploaded in different order!
	sort.Slice(completedParts, func(i, j int) bool {
		return *completedParts[i].PartNumber < *completedParts[j].PartNumber
	})

	// Signalling AWS S3 that the multiPartUpload is finished
	completeMultipartOuput, err := client.Client.CompleteMultipartUpload(context.TODO(), &s3.CompleteMultipartUploadInput{
		Bucket:   createUploadOuput.Bucket,
		Key:      createUploadOuput.Key,
		UploadId: createUploadOuput.UploadId,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	})

	if err != nil {
		client.Logger.Error(fmt.Sprintf(
			"couldn't complete multipart copy object to %v:%v from %v. Error: %v",
			bucketName, objectKey, source, err,
		))
		return nil, err
	} else {
		return completeMultipartOuput, nil
	}
}

// UploadObject uses an upload manager to upload data from a reader to an object in a bucket.
// The upload manager leverages S3 SDK multipart uploads for large contents to stream content from the reader in
// parallelized chunks
func (client *S3) UploadObject(
	bucketName string, objectKey string, reader io.Reader, metadata map[string]string, contentType *string) (*manager.UploadOutput, error) {
	var partMiBs int64 = 10
	uploader := manager.NewUploader(client.Client, func(u *manager.Uploader) {
		u.PartSize = partMiBs * 1024 * 1024
	})
	resp, err := uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(objectKey),
		Body:        reader,
		Metadata:    metadata,
		ContentType: contentType,
	})
	if err != nil {
		client.Logger.Error(fmt.Sprintf("could not upload large stream to %v:%v. Error: %v",
			bucketName, objectKey, err))
	}

	return resp, err
}

// DownloadObject makes a pre-signed request that can be used to get an object from a bucket.
// The pre-signed request is valid for the specified number of seconds.
func (client *S3) DownloadObject(
	bucketName string, objectKey string, downloadName string, lifetimeSecs int64) (*v4.PresignedHTTPRequest, error) {
	request, err := client.PresignClient.PresignGetObject(
		context.TODO(),
		&s3.GetObjectInput{
			Bucket:                     aws.String(bucketName),
			Key:                        aws.String(objectKey),
			ResponseContentDisposition: aws.String(fmt.Sprintf(`attachment; filename="%s"`, downloadName)),
		}, func(opts *s3.PresignOptions) {
			opts.Expires = time.Duration(lifetimeSecs * int64(time.Second))
		},
	)
	if err != nil {
		client.Logger.Error(fmt.Sprintf("could not get a presigned request to get %v:%v. error: %v",
			bucketName, objectKey, err))
	}
	return request, err
}

// ObjectExists checks if an object exists in the S3 bucket.
func (client *S3) ObjectExists(bucketName string, objectKey string) (bool, error) {
	_, err := client.Client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		if strings.Contains(err.Error(), "S3: HeadObject") &&
			strings.Contains(err.Error(), "StatusCode: 404") {
			return false, nil
		}

		client.Logger.Error(
			fmt.Sprintf("failed to check for existing object in S3 '%v:%v'. error: %v",
				bucketName, objectKey, err),
		)
		return false, err
	}

	return true, nil
}

// DeleteObjects deletes a list of objects from a bucket.
func (client *S3) DeleteObjects(bucketName string, objectKeys []string) (*s3.DeleteObjectsOutput, error) {
	var objectIds []types.ObjectIdentifier
	for _, key := range objectKeys {
		objectIds = append(objectIds, types.ObjectIdentifier{Key: aws.String(key)})
	}
	resp, err := client.Client.DeleteObjects(context.TODO(), &s3.DeleteObjectsInput{
		Bucket: aws.String(bucketName),
		Delete: &types.Delete{Objects: objectIds},
	})
	if err != nil {
		client.Logger.Error(fmt.Sprintf("could not delete objects from bucket %v. Error: %v", bucketName, err))
	}
	return resp, err
}

// DeleteVersionedObject deletes a version of an object from the specified bucket.
func (client *S3) DeleteVersionedObject(bucketName string, objectKey string, objectVersion string) (*s3.DeleteObjectsOutput, error) {
	resp, err := client.Client.DeleteObjects(context.TODO(), &s3.DeleteObjectsInput{
		Bucket: aws.String(bucketName),
		Delete: &types.Delete{Objects: []types.ObjectIdentifier{
			{
				Key:       aws.String(objectKey),
				VersionId: aws.String(objectVersion),
			},
		}},
	})
	if err != nil {
		client.Logger.Error(fmt.Sprintf(
			"could not delete object with key:%s & version:%s from bucket %v. Error: %v",
			objectKey, objectVersion, bucketName, err,
		))
	}
	return resp, err
}

func (client *S3) uploadMultipartToS3(resp *s3.CreateMultipartUploadOutput, source string, start int, end int, partNum int, ch chan partUploadResult, wg *sync.WaitGroup) {
	defer wg.Done()
	var try int
	sourceRange := fmt.Sprintf("bytes=%d-%d", start, end-1)
	for try <= retries {
		client.Logger.Info(fmt.Sprintf("uploading part %v to %v:[%s] \n", partNum, source, sourceRange))
		uploadRes, err := client.Client.UploadPartCopy(context.TODO(), &s3.UploadPartCopyInput{
			Bucket:          resp.Bucket,
			Key:             resp.Key,
			PartNumber:      aws.Int32(int32(partNum)),
			UploadId:        resp.UploadId,
			CopySource:      aws.String(source),
			CopySourceRange: aws.String(sourceRange),
		})
		if err != nil {
			client.Logger.Error(fmt.Sprintf(
				"couldn't upload part of multipart copy object to %v:[%s]. Error: %v",
				source, sourceRange, err,
			))
			if try == retries {
				ch <- partUploadResult{nil, err}
				return
			} else {
				try++
				time.Sleep(time.Second * 15)
			}
		} else {
			client.Logger.Info(fmt.Sprintf("part %v has been uploaded \n", partNum))
			ch <- partUploadResult{
				&types.CompletedPart{
					ETag:       uploadRes.CopyPartResult.ETag,
					PartNumber: aws.Int32(int32(partNum)),
				}, nil,
			}
			return
		}
	}
	ch <- partUploadResult{}
}
