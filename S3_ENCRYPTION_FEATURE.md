# S3 Customer-Managed Keys Support

This document de3. In the "Encryption Settings" section:
   - Select the desired encryption type
   - If using AWS KMS, provide the KMS Key ID (ARN or key ID)
   - If using SSE-C, provide a base64-encoded 256-bit encryption key
   - For KMS, optionally specify the key regionbes the S3 server-side encryption support added to pgbackweb.

## Overview

pgbackweb now supports four types of server-side encryption for S3 destinations:

1. **None** - No encryption (default for backward compatibility)
2. **AES256** - S3-managed encryption using AES-256
3. **AWS KMS** - Customer-managed encryption using AWS Key Management Service
4. **SSE-C** - Server-Side Encryption with Customer-Provided Keys (universal S3 compatibility)

## S3-Compatible Provider Support

The **SSE-C** encryption type provides universal support for S3-compatible providers such as:
- **Hetzner Cloud Storage**
- **DigitalOcean Spaces**
- **Linode Object Storage**
- **Wasabi**
- **MinIO**
- **Any S3-compatible storage provider**

This allows you to use your own encryption keys with any S3-compatible service, not just AWS.

## Configuration

### Database Schema

The `destinations` table has been extended with three new columns:

- `encryption_type` (TEXT, NOT NULL, DEFAULT 'none'): The encryption method
- `encryption_key_id` (TEXT, NULLABLE): KMS key ID or ARN (required for aws:kms)
- `encryption_key_region` (TEXT, NULLABLE): KMS key region (optional)

### Web Interface

The destination creation and editing forms now include:

- **Encryption Type** dropdown with options: "No encryption", "AES-256 (S3 managed)", "AWS KMS", "SSE-C (Customer-provided key)"
- **Encryption Key** field that accepts:
  - KMS key ARN or ID (for AWS KMS)
  - Base64-encoded 256-bit encryption key (for SSE-C)
- **KMS Key Region** field for specifying the key region (optional, AWS KMS only)

## Usage

### Creating a Destination with Encryption

1. Navigate to Destinations in the dashboard
2. Click "Add destination"
3. Fill in the standard S3 configuration fields
4. In the "Encryption Settings" section:
   - Select the desired encryption type
   - If using AWS KMS, provide the KMS Key ID (ARN or key ID)
   - Optionally specify the KMS key region

### Encryption Types

#### No Encryption
- Default option for backward compatibility
- Objects are stored without server-side encryption

#### AES-256 (S3 Managed)
- Uses S3's built-in AES-256 encryption
- No additional configuration required
- Keys are managed by AWS S3

#### SSE-C (Server-Side Encryption with Customer-Provided Keys)
- Uses customer-provided encryption keys
- Works with any S3-compatible provider (Hetzner, DigitalOcean, etc.)
- Requires a base64-encoded 256-bit (32-byte) encryption key
- Keys are not stored by the provider - you must manage them
- Provides maximum control over encryption keys

#### AWS KMS
- Uses customer-managed KMS keys
- Requires a valid KMS key ID or ARN
- Supports cross-region keys
- Provides additional audit trails and access controls
- AWS-specific feature

## Implementation Details

### S3 Client Changes

- Added `S3UploadWithEncryption` method to support encryption parameters
- Added `S3GetDownloadLinkWithEncryption` method for encrypted downloads
- Original methods maintained for backward compatibility
- **Four encryption types supported**:
  - `none` - No encryption (default)
  - `aes256` - S3-managed AES-256 encryption
  - `aws:kms` - Customer-managed KMS keys
  - `sse-c` - Customer-provided keys (universal S3 compatibility)
- Uses AWS SDK v2 encryption types for type safety

### Database Migration

Migration `20250714000000_add_s3_encryption_support.sql` adds:
- New encryption columns to destinations table
- Constraints to ensure valid encryption configuration
- Index for encryption type lookups

### Service Layer

- `DestinationsService.CreateDestination` sets default encryption type to "none"
- `ExecutionsService.RunExecution` passes encryption parameters to S3 client
- Form validation ensures KMS key ID is provided when using AWS KMS

## Security Considerations

1. **KMS Key Access**: Ensure the AWS credentials have access to the specified KMS key
2. **Key Rotation**: KMS keys can be rotated without affecting existing encrypted objects
3. **Cross-Region Keys**: When using KMS keys from different regions, ensure proper IAM permissions
4. **Audit Trails**: KMS encryption provides detailed CloudTrail logs for key usage
5. **SSE-C Key Management**: 
   - **Critical**: Store SSE-C keys securely - losing them means losing access to your data
   - Use a secure key management system or password manager
   - Consider key rotation policies
   - Back up your keys in multiple secure locations
6. **S3-Compatible Provider Security**: Verify that your chosen provider supports SSE-C properly
7. **Key Transmission**: Keys are transmitted securely over HTTPS but never stored by the provider

## Examples

### Generating an SSE-C Encryption Key

```bash
# Generate a random 256-bit key and encode it as base64
openssl rand -base64 32
# Example output: YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXpBQkNERUY=
```

### S3-Compatible Provider Examples

#### Hetzner Cloud Storage
```
Endpoint: https://fsn1.your-objectstorage.com
Region: fsn1
Encryption Type: SSE-C
Encryption Key: YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXpBQkNERUY=
```

#### DigitalOcean Spaces
```
Endpoint: https://nyc3.digitaloceanspaces.com
Region: nyc3
Encryption Type: SSE-C
Encryption Key: YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXpBQkNERUY=
```

#### MinIO
```
Endpoint: https://your-minio-server.com
Region: us-east-1
Encryption Type: SSE-C
Encryption Key: YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXpBQkNERUY=
```

### KMS Key ID Formats

```
# Key ID
12345678-1234-1234-1234-123456789012

# Key ARN
arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012

# Alias
alias/my-key
```

### IAM Permissions

For AWS KMS encryption, ensure your AWS credentials have the following permissions:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "kms:Encrypt",
                "kms:Decrypt",
                "kms:GenerateDataKey"
            ],
            "Resource": "arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012"
        }
    ]
}
```

## Testing

Run the encryption tests:

```bash
cd /path/to/pgbackweb
go test ./internal/integration/storage/ -v
```

Note: The tests require valid AWS credentials and an accessible S3 bucket for full integration testing.

## Backward Compatibility

- Existing destinations without encryption settings will default to "none"
- The original `S3Upload` function is preserved for backward compatibility
- No changes required for existing backup configurations

## Troubleshooting

### Common Issues

1. **KMS Key Not Found**: Ensure the key ID/ARN is correct and the region is accessible
2. **Permission Denied**: Verify IAM permissions for the KMS key
3. **Cross-Region Access**: Check that the KMS key region is correctly specified
4. **Invalid Key Format**: Use the correct format for key ID, ARN, or alias

### Logs

Check the application logs for encryption-related errors:
- Key access denied
- Invalid key format
- Region mismatch
- Network connectivity issues
