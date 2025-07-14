package databases

import (
	"context"
	"fmt"
	"net/url"

	"github.com/eduardolat/pgbackweb/internal/database/dbgen"
	"github.com/google/uuid"
)

// BulkImportRequest represents a request for bulk importing databases
type BulkImportRequest struct {
	ConnectionString string
	Version          string
	SelectedDatabases []string
}

// DatabaseDiscovery represents discovered database information
type DatabaseDiscovery struct {
	Name    string
	Size    string
	Owner   string
	Comment string
}

// DiscoverDatabases discovers databases from a PostgreSQL server
func (s *Service) DiscoverDatabases(
	ctx context.Context, version, connString string,
) ([]DatabaseDiscovery, error) {
	pgVersion, err := s.ints.PGClient.ParseVersion(version)
	if err != nil {
		return nil, fmt.Errorf("error parsing PostgreSQL version: %w", err)
	}

	// Test connection first
	err = s.ints.PGClient.Test(pgVersion, connString)
	if err != nil {
		return nil, fmt.Errorf("error testing database connection: %w", err)
	}

	// Query databases
	databases, err := s.ints.PGClient.QueryDatabases(pgVersion, connString)
	if err != nil {
		return nil, fmt.Errorf("error querying databases: %w", err)
	}

	// Convert to our internal type
	var result []DatabaseDiscovery
	for _, db := range databases {
		result = append(result, DatabaseDiscovery{
			Name:    db.Name,
			Size:    db.Size,
			Owner:   db.Owner,
			Comment: db.Comment,
		})
	}

	return result, nil
}

// BulkImportDatabases imports multiple databases from a single connection
func (s *Service) BulkImportDatabases(
	ctx context.Context, req BulkImportRequest,
) error {
	if len(req.SelectedDatabases) == 0 {
		return fmt.Errorf("no databases selected for import")
	}

	// Parse the base connection string
	u, err := url.Parse(req.ConnectionString)
	if err != nil {
		return fmt.Errorf("invalid connection string: %w", err)
	}

	// Create individual database connections and import them
	for _, dbName := range req.SelectedDatabases {
		// Create connection string for this specific database
		dbConnString := s.buildDatabaseConnectionString(u, dbName)
		
		// Test the connection for this database
		err = s.TestDatabase(ctx, req.Version, dbConnString)
		if err != nil {
			return fmt.Errorf("error testing database %s: %w", dbName, err)
		}

		// Import the database
		_, err = s.dbgen.DatabasesServiceBulkImportDatabase(ctx, dbgen.DatabasesServiceBulkImportDatabaseParams{
			Name:             dbName,
			ConnectionString: dbConnString,
			PgVersion:        req.Version,
			EncryptionKey:    s.env.PBW_ENCRYPTION_KEY,
		})
		if err != nil {
			return fmt.Errorf("error importing database %s: %w", dbName, err)
		}
	}

	// Test all databases in the background
	go func() {
		databases, err := s.dbgen.DatabasesServiceGetAllDatabases(ctx, s.env.PBW_ENCRYPTION_KEY)
		if err != nil {
			return
		}
		
		for _, db := range databases {
			for _, selectedDB := range req.SelectedDatabases {
				if db.Name == selectedDB {
					_ = s.TestDatabaseAndStoreResult(ctx, db.ID)
					break
				}
			}
		}
	}()

	return nil
}

// buildDatabaseConnectionString creates a connection string for a specific database
func (s *Service) buildDatabaseConnectionString(u *url.URL, dbName string) string {
	// Create a copy of the URL
	newURL := *u
	
	// Update the path to include the database name
	newURL.Path = "/" + dbName
	
	return newURL.String()
}

// BulkUpdateConnectionStringRequest represents a request for bulk updating connection strings
type BulkUpdateConnectionStringRequest struct {
	DatabaseIDs           []string `json:"database_ids"`
	NewConnectionString   string   `json:"new_connection_string"`
	OnlyUpdateHost        bool     `json:"only_update_host"`
	OnlyUpdateCredentials bool     `json:"only_update_credentials"`
}

// BulkUpdateConnectionString updates connection strings for multiple databases
func (s *Service) BulkUpdateConnectionString(
	ctx context.Context, req BulkUpdateConnectionStringRequest,
) error {
	if len(req.DatabaseIDs) == 0 {
		return fmt.Errorf("no databases selected for update")
	}

	// Convert string IDs to UUIDs
	var databaseUUIDs []uuid.UUID
	for _, idStr := range req.DatabaseIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			return fmt.Errorf("invalid database ID: %s", idStr)
		}
		databaseUUIDs = append(databaseUUIDs, id)
	}

	// If partial update is requested, we need to get existing connection strings
	// and merge them with the new values
	if req.OnlyUpdateHost || req.OnlyUpdateCredentials {
		return s.bulkUpdateConnectionStringPartial(ctx, databaseUUIDs, req)
	}

	// Full connection string replacement
	return s.bulkUpdateConnectionStringFull(ctx, databaseUUIDs, req.NewConnectionString)
}

// bulkUpdateConnectionStringFull replaces the entire connection string
func (s *Service) bulkUpdateConnectionStringFull(
	ctx context.Context, databaseUUIDs []uuid.UUID, newConnectionString string,
) error {
	// Test the new connection string with a sample database
	if len(databaseUUIDs) > 0 {
		// Get the first database to test the connection
		db, err := s.GetDatabase(ctx, databaseUUIDs[0])
		if err != nil {
			return fmt.Errorf("error getting database for testing: %w", err)
		}

		// Test the new connection string
		err = s.TestDatabase(ctx, db.PgVersion, newConnectionString)
		if err != nil {
			return fmt.Errorf("error testing new connection string: %w", err)
		}
	}

	// Update all databases
	err := s.dbgen.DatabasesServiceBulkUpdateConnectionString(ctx, dbgen.DatabasesServiceBulkUpdateConnectionStringParams{
		DatabaseIds:         databaseUUIDs,
		NewConnectionString: newConnectionString,
		EncryptionKey:       s.env.PBW_ENCRYPTION_KEY,
	})
	if err != nil {
		return fmt.Errorf("error updating connection strings: %w", err)
	}

	// Test all databases in the background
	go func() {
		for _, dbID := range databaseUUIDs {
			_ = s.TestDatabaseAndStoreResult(ctx, dbID)
		}
	}()

	return nil
}

// bulkUpdateConnectionStringPartial updates only specific parts of the connection string
func (s *Service) bulkUpdateConnectionStringPartial(
	ctx context.Context, databaseUUIDs []uuid.UUID, req BulkUpdateConnectionStringRequest,
) error {
	// Parse the new connection string to extract components
	newURL, err := url.Parse(req.NewConnectionString)
	if err != nil {
		return fmt.Errorf("invalid new connection string: %w", err)
	}

	// Get all affected databases
	databases, err := s.dbgen.DatabasesServiceGetAllDatabases(ctx, s.env.PBW_ENCRYPTION_KEY)
	if err != nil {
		return fmt.Errorf("error getting databases: %w", err)
	}

	// Filter to only the databases we're updating
	var targetDatabases []dbgen.DatabasesServiceGetAllDatabasesRow
	for _, db := range databases {
		for _, targetID := range databaseUUIDs {
			if db.ID == targetID {
				targetDatabases = append(targetDatabases, db)
				break
			}
		}
	}

	// Update each database individually
	for _, db := range targetDatabases {
		// Parse existing connection string
		existingURL, err := url.Parse(db.DecryptedConnectionString)
		if err != nil {
			return fmt.Errorf("error parsing existing connection string for database %s: %w", db.Name, err)
		}

		// Create updated connection string
		updatedURL := *existingURL

		if req.OnlyUpdateHost {
			// Update only host and port
			updatedURL.Host = newURL.Host
		}

		if req.OnlyUpdateCredentials {
			// Update only username and password
			updatedURL.User = newURL.User
		}

		// Test the updated connection string
		err = s.TestDatabase(ctx, db.PgVersion, updatedURL.String())
		if err != nil {
			return fmt.Errorf("error testing updated connection string for database %s: %w", db.Name, err)
		}

		// Update the database
		err = s.dbgen.DatabasesServiceBulkUpdateConnectionString(ctx, dbgen.DatabasesServiceBulkUpdateConnectionStringParams{
			DatabaseIds:         []uuid.UUID{db.ID},
			NewConnectionString: updatedURL.String(),
			EncryptionKey:       s.env.PBW_ENCRYPTION_KEY,
		})
		if err != nil {
			return fmt.Errorf("error updating connection string for database %s: %w", db.Name, err)
		}
	}

	// Test all databases in the background
	go func() {
		for _, dbID := range databaseUUIDs {
			_ = s.TestDatabaseAndStoreResult(ctx, dbID)
		}
	}()

	return nil
}
