package storage

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/eduardolat/pgbackweb/internal/util/strutil"
)

// createS3Client creates a new S3 client
func createS3Client(
	accessKey, secretKey, region, endpoint string,
) (*s3.Client, error) {
	credentialsProvider := credentials.NewStaticCredentialsProvider(
		accessKey, secretKey, "",
	)

	//nolint:all
	endpointResolver := aws.EndpointResolverFunc(func(
		_ string, _ string,
	) (aws.Endpoint, error) {
		return aws.Endpoint{
			HostnameImmutable: true,
			URL:               endpoint,
		}, nil
	})

	//nolint:all
	conf, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithRegion(region),
		config.WithEndpointResolver(endpointResolver),
		config.WithCredentialsProvider(credentialsProvider),
	)
	if err != nil {
		return nil, fmt.Errorf("error initializing storage config: %w", err)
	}

	s3Client := s3.NewFromConfig(conf)
	return s3Client, nil
}

// S3Test tests the connection to S3
func (Client) S3Test(
	accessKey, secretKey, region, endpoint, bucketName string,
) error {
	s3Client, err := createS3Client(
		accessKey, secretKey, region, endpoint,
	)
	if err != nil {
		return err
	}

	_, err = s3Client.HeadBucket(
		context.TODO(),
		&s3.HeadBucketInput{
			Bucket: aws.String(bucketName),
		},
	)
	if err != nil {
		return fmt.Errorf("failed to test S3 bucket: %w", err)
	}

	return nil
}

// S3Upload uploads a file to S3 from a reader.
//
// Returns the file size, in bytes.
func (c Client) S3Upload(
	accessKey, secretKey, region, endpoint, bucketName, key string,
	fileReader io.Reader,
) (int64, error) {
	return c.S3UploadWithEncryption(
		accessKey, secretKey, region, endpoint, bucketName, key,
		fileReader, "", "", "",
	)
}

// S3UploadWithEncryption uploads a file to S3 from a reader with encryption support.
//
// encryptionType can be: "none", "aes256", "aws:kms", or "sse-c"
// encryptionKeyId:
//   - For aws:kms: KMS key ID or ARN
//   - For sse-c: base64-encoded 256-bit encryption key
// encryptionKeyRegion is the region for the KMS key (optional, defaults to bucket region)
//
// Returns the file size, in bytes.
func (c Client) S3UploadWithEncryption(
	accessKey, secretKey, region, endpoint, bucketName, key string,
	fileReader io.Reader, encryptionType, encryptionKeyId, encryptionKeyRegion string,
) (int64, error) {
	s3Client, err := createS3Client(
		accessKey, secretKey, region, endpoint,
	)
	if err != nil {
		return 0, err
	}

	key = strutil.RemoveLeadingSlash(key)
	contentType := strutil.GetContentTypeFromFileName(key)

	// Prepare the PutObjectInput
	putObjectInput := &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(key),
		Body:        fileReader,
		ContentType: aws.String(contentType),
	}

	// Add encryption parameters based on encryption type
	switch encryptionType {
	case "aes256":
		putObjectInput.ServerSideEncryption = types.ServerSideEncryptionAes256
	case "aws:kms":
		putObjectInput.ServerSideEncryption = types.ServerSideEncryptionAwsKms
		if encryptionKeyId != "" {
			putObjectInput.SSEKMSKeyId = aws.String(encryptionKeyId)
		}
		// Note: encryptionKeyRegion is not a direct parameter for PutObject
		// The key region is handled at the KMS key level
	case "sse-c":
		if encryptionKeyId == "" {
			return 0, fmt.Errorf("encryption key is required for SSE-C")
		}
		
		// Decode the base64-encoded key
		keyBytes, err := base64.StdEncoding.DecodeString(encryptionKeyId)
		if err != nil {
			return 0, fmt.Errorf("invalid base64 encryption key: %w", err)
		}
		
		if len(keyBytes) != 32 {
			return 0, fmt.Errorf("encryption key must be 32 bytes (256 bits)")
		}
		
		// Calculate SHA256 hash of the key
		keyHash := sha256.Sum256(keyBytes)
		
		putObjectInput.SSECustomerAlgorithm = aws.String("AES256")
		putObjectInput.SSECustomerKey = aws.String(encryptionKeyId)
		putObjectInput.SSECustomerKeyMD5 = aws.String(base64.StdEncoding.EncodeToString(keyHash[:]))
	}

	uploader := manager.NewUploader(s3Client)
	_, err = uploader.Upload(
		context.TODO(),
		putObjectInput,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to upload file to S3: %w", err)
	}

	fileHead, err := s3Client.HeadObject(
		context.TODO(),
		&s3.HeadObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(key),
		},
	)
	if err != nil {
		return 0, fmt.Errorf("failed to get uploaded file info from S3: %w", err)
	}

	var fileSize int64
	if fileHead.ContentLength != nil {
		fileSize = *fileHead.ContentLength
	}

	return fileSize, nil
}

// S3Delete deletes a file from S3
func (Client) S3Delete(
	accessKey, secretKey, region, endpoint, bucketName, key string,
) error {
	s3Client, err := createS3Client(
		accessKey, secretKey, region, endpoint,
	)
	if err != nil {
		return err
	}

	key = strutil.RemoveLeadingSlash(key)

	_, err = s3Client.DeleteObject(
		context.TODO(),
		&s3.DeleteObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(key),
		},
	)
	if err != nil {
		return fmt.Errorf("failed to delete file from S3: %w", err)
	}

	return nil
}

// S3GetDownloadLink generates a presigned URL for downloading a file from S3
func (c Client) S3GetDownloadLink(
	accessKey, secretKey, region, endpoint, bucketName, key string,
	expiration time.Duration,
) (string, error) {
	return c.S3GetDownloadLinkWithEncryption(
		accessKey, secretKey, region, endpoint, bucketName, key,
		expiration, "", "", "",
	)
}

// S3GetDownloadLinkWithEncryption generates a presigned URL for downloading a file from S3 with encryption support
func (c Client) S3GetDownloadLinkWithEncryption(
	accessKey, secretKey, region, endpoint, bucketName, key string,
	expiration time.Duration, encryptionType, encryptionKeyId, encryptionKeyRegion string,
) (string, error) {
	s3Client, err := createS3Client(
		accessKey, secretKey, region, endpoint,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create S3 client: %w", err)
	}

	getObjectInput := &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	}

	// Add encryption parameters for SSE-C
	if encryptionType == "sse-c" && encryptionKeyId != "" {
		// Decode the base64-encoded key
		keyBytes, err := base64.StdEncoding.DecodeString(encryptionKeyId)
		if err != nil {
			return "", fmt.Errorf("invalid base64 encryption key: %w", err)
		}
		
		if len(keyBytes) != 32 {
			return "", fmt.Errorf("encryption key must be 32 bytes (256 bits)")
		}
		
		// Calculate SHA256 hash of the key
		keyHash := sha256.Sum256(keyBytes)
		
		getObjectInput.SSECustomerAlgorithm = aws.String("AES256")
		getObjectInput.SSECustomerKey = aws.String(encryptionKeyId)
		getObjectInput.SSECustomerKeyMD5 = aws.String(base64.StdEncoding.EncodeToString(keyHash[:]))
	}

	presigned, err := s3.NewPresignClient(s3Client).PresignGetObject(
		context.TODO(),
		getObjectInput,
		s3.WithPresignExpires(expiration),
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return presigned.URL, nil
}
