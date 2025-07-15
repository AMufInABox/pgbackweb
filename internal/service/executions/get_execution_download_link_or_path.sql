-- name: ExecutionsServiceGetDownloadLinkOrPathData :one
SELECT
  executions.path AS path,
  backups.is_local AS is_local,
  destinations.bucket_name AS bucket_name,
  destinations.region AS region,
  destinations.endpoint AS endpoint,
  destinations.endpoint as destination_endpoint,
  destinations.encryption_type as destination_encryption_type,
  destinations.encryption_key_id as destination_encryption_key_id,
  destinations.encryption_key_region as destination_encryption_key_region,
  (
    CASE WHEN destinations.access_key IS NOT NULL
    THEN pgp_sym_decrypt(destinations.access_key, sqlc.arg('decryption_key')::TEXT)
    ELSE ''
    END
  ) AS decrypted_access_key,
  (
    CASE WHEN destinations.secret_key IS NOT NULL
    THEN pgp_sym_decrypt(destinations.secret_key, sqlc.arg('decryption_key')::TEXT)
    ELSE ''
    END
  ) AS decrypted_secret_key
FROM executions
INNER JOIN backups ON backups.id = executions.backup_id
LEFT JOIN destinations ON destinations.id = backups.destination_id
WHERE executions.id = @execution_id;
