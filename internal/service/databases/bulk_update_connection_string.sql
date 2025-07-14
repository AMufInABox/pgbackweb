-- name: DatabasesServiceBulkUpdateConnectionString :exec
UPDATE databases 
SET 
  connection_string = pgp_sym_encrypt(@new_connection_string, @encryption_key),
  updated_at = NOW()
WHERE id = ANY(@database_ids::uuid[]);
