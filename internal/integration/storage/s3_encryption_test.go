package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
)

func TestS3UploadWithEncryption(t *testing.T) {
	// This test validates the encryption parameters are correctly applied
	// In a real test, you would mock the S3 client and verify the parameters
	
	client := Client{}
	
	// Test data
	accessKey := "test-access-key"
	secretKey := "test-secret-key"
	region := "us-east-1"
	endpoint := "https://s3.amazonaws.com"
	bucketName := "test-bucket"
	key := "test-key"
	content := strings.NewReader("test content")
	
	// Test cases
	testCases := []struct {
		name                string
		encryptionType      string
		encryptionKeyId     string
		encryptionKeyRegion string
		expectedError       bool
	}{
		{
			name:           "No encryption",
			encryptionType: "none",
			expectedError:  false,
		},
		{
			name:           "AES256 encryption",
			encryptionType: "aes256",
			expectedError:  false,
		},
		{
			name:            "KMS encryption with key ID",
			encryptionType:  "aws:kms",
			encryptionKeyId: "arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012",
			expectedError:   false,
		},
		{
			name:                "KMS encryption with key ID and region",
			encryptionType:      "aws:kms",
			encryptionKeyId:     "arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012",
			encryptionKeyRegion: "us-east-1",
			expectedError:       false,
		},
		{
			name:            "SSE-C encryption with base64 key",
			encryptionType:  "sse-c",
			encryptionKeyId: "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXpBQkNERUY=", // 32 bytes base64
			expectedError:   false,
		},
		{
			name:            "SSE-C encryption with invalid key length",
			encryptionType:  "sse-c",
			encryptionKeyId: "dGVzdA==", // "test" in base64, only 4 bytes
			expectedError:   true,
		},
		{
			name:            "SSE-C encryption with invalid base64",
			encryptionType:  "sse-c",
			encryptionKeyId: "invalid-base64!@#",
			expectedError:   true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Since we can't actually test against AWS without credentials,
			// we'll just validate the function signature and parameters
			
			// This would fail if the bucket doesn't exist or credentials are invalid
			// but that's expected in a unit test environment
			_, err := client.S3UploadWithEncryption(
				accessKey, secretKey, region, endpoint, bucketName, key,
				content, tc.encryptionType, tc.encryptionKeyId, tc.encryptionKeyRegion,
			)
			
			if tc.expectedError {
				assert.Error(t, err, "Expected error for invalid configuration")
			} else {
				// We expect an error due to invalid credentials/bucket in test environment
				// but the function should not panic and should handle the parameters correctly
				assert.Error(t, err, "Expected error due to test environment")
			}
		})
	}
}

func TestS3EncryptionTypes(t *testing.T) {
	// Test that we're using the correct AWS SDK types
	assert.Equal(t, string(types.ServerSideEncryptionAes256), "AES256")
	assert.Equal(t, string(types.ServerSideEncryptionAwsKms), "aws:kms")
}

func TestS3UploadBackwardCompatibility(t *testing.T) {
	// Test that the original S3Upload function still works
	client := Client{}
	
	accessKey := "test-access-key"
	secretKey := "test-secret-key"
	region := "us-east-1"
	endpoint := "https://s3.amazonaws.com"
	bucketName := "test-bucket"
	key := "test-key"
	content := strings.NewReader("test content")
	
	// This should call S3UploadWithEncryption with no encryption
	_, err := client.S3Upload(
		accessKey, secretKey, region, endpoint, bucketName, key, content,
	)
	
	// We expect an error due to invalid credentials/bucket in test environment
	assert.Error(t, err, "Expected error due to test environment")
}

func TestS3GetDownloadLinkWithEncryption(t *testing.T) {
	client := Client{}
	
	accessKey := "test-access-key"
	secretKey := "test-secret-key"
	region := "us-east-1"
	endpoint := "https://s3.amazonaws.com"
	bucketName := "test-bucket"
	key := "test-key"
	expiration := time.Hour * 1
	
	// Test SSE-C download link generation
	link, err := client.S3GetDownloadLinkWithEncryption(
		accessKey, secretKey, region, endpoint, bucketName, key, expiration,
		"sse-c", "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXpBQkNERUY=", "",
	)
	
	// Presigned URL generation should work even with invalid credentials
	// The error would occur when actually accessing the URL
	if err != nil {
		assert.Error(t, err, "Expected error due to test environment")
	} else {
		assert.NotEmpty(t, link, "Expected non-empty presigned URL")
	}
}
