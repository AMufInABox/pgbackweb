package databases

import (
	"context"
	"fmt"
	"net/url"

	"github.com/eduardolat/pgbackweb/internal/database/dbgen"
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
