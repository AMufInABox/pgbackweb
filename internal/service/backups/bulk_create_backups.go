package backups

import (
	"context"
	"fmt"
	"strings"

	"github.com/eduardolat/pgbackweb/internal/database/dbgen"
	"github.com/eduardolat/pgbackweb/internal/validate"
	"github.com/google/uuid"
)

// BulkCreateBackupRequest represents a request for bulk backup creation
type BulkCreateBackupRequest struct {
	DatabaseIDs    []string   `json:"database_ids"`
	DestinationID  *uuid.UUID `json:"destination_id"`
	IsLocal        bool       `json:"is_local"`
	NameTemplate   string     `json:"name_template"`
	CronExpression string     `json:"cron_expression"`
	TimeZone       string     `json:"time_zone"`
	IsActive       bool       `json:"is_active"`
	DestDir        string     `json:"dest_dir"`
	RetentionDays  int16      `json:"retention_days"`
	OptDataOnly    bool       `json:"opt_data_only"`
	OptSchemaOnly  bool       `json:"opt_schema_only"`
	OptClean       bool       `json:"opt_clean"`
	OptIfExists    bool       `json:"opt_if_exists"`
	OptCreate      bool       `json:"opt_create"`
	OptNoComments  bool       `json:"opt_no_comments"`
}

// BulkCreateBackupResponse represents the response from bulk backup creation
type BulkCreateBackupResponse struct {
	CreatedBackups []dbgen.Backup `json:"created_backups"`
	Errors         []string       `json:"errors"`
}

// BulkCreateBackups creates backup tasks for multiple databases
func (s *Service) BulkCreateBackups(
	ctx context.Context, req BulkCreateBackupRequest,
) (BulkCreateBackupResponse, error) {
	response := BulkCreateBackupResponse{
		CreatedBackups: []dbgen.Backup{},
		Errors:         []string{},
	}

	if len(req.DatabaseIDs) == 0 {
		return response, fmt.Errorf("no databases selected for backup creation")
	}

	// Validate cron expression
	if !validate.CronExpression(req.CronExpression) {
		return response, fmt.Errorf("invalid cron expression")
	}

	// Get database information for name generation
	databases, err := s.dbgen.DatabasesServiceGetAllDatabases(ctx, s.env.PBW_ENCRYPTION_KEY)
	if err != nil {
		return response, fmt.Errorf("error getting databases: %w", err)
	}

	// Create a map for quick database lookup
	databaseMap := make(map[string]dbgen.DatabasesServiceGetAllDatabasesRow)
	for _, db := range databases {
		databaseMap[db.ID.String()] = db
	}

	// Create backups for each selected database
	for _, dbIDStr := range req.DatabaseIDs {
		dbID, err := uuid.Parse(dbIDStr)
		if err != nil {
			response.Errors = append(response.Errors, fmt.Sprintf("Invalid database ID: %s", dbIDStr))
			continue
		}

		// Get database info for name generation
		database, exists := databaseMap[dbID.String()]
		if !exists {
			response.Errors = append(response.Errors, fmt.Sprintf("Database not found: %s", dbIDStr))
			continue
		}

		// Generate backup name from template
		backupName := s.generateBackupName(req.NameTemplate, database.Name)

		// Create database-specific subdirectory
		databaseDestDir := s.generateDatabaseDestDir(req.DestDir, database.Name)

		// Create backup parameters
		params := dbgen.BackupsServiceBulkCreateBackupParams{
			DatabaseID: dbID,
			DestinationID: uuid.NullUUID{Valid: req.DestinationID != nil, UUID: func() uuid.UUID {
				if req.DestinationID != nil {
					return *req.DestinationID
				}
				return uuid.UUID{}
			}()},
			IsLocal:        req.IsLocal,
			Name:           backupName,
			CronExpression: req.CronExpression,
			TimeZone:       req.TimeZone,
			IsActive:       req.IsActive,
			DestDir:        databaseDestDir,
			RetentionDays:  req.RetentionDays,
			OptDataOnly:    req.OptDataOnly,
			OptSchemaOnly:  req.OptSchemaOnly,
			OptClean:       req.OptClean,
			OptIfExists:    req.OptIfExists,
			OptCreate:      req.OptCreate,
			OptNoComments:  req.OptNoComments,
		}

		// Create the backup
		backup, err := s.dbgen.BackupsServiceBulkCreateBackup(ctx, params)
		if err != nil {
			response.Errors = append(response.Errors, fmt.Sprintf("Error creating backup for database %s: %v", database.Name, err))
			continue
		}

		// Schedule the backup job if it's active
		if backup.IsActive {
			err = s.jobUpsert(backup.ID, backup.TimeZone, backup.CronExpression)
			if err != nil {
				response.Errors = append(response.Errors, fmt.Sprintf("Error scheduling backup job for database %s: %v", database.Name, err))
				// Continue with creation even if scheduling fails
			}
		}

		response.CreatedBackups = append(response.CreatedBackups, backup)
	}

	return response, nil
}

// generateDatabaseDestDir creates a database-specific destination directory
func (s *Service) generateDatabaseDestDir(baseDestDir, databaseName string) string {
	if baseDestDir == "" {
		return databaseName
	}

	// Remove trailing slash if present
	if strings.HasSuffix(baseDestDir, "/") {
		baseDestDir = baseDestDir[:len(baseDestDir)-1]
	}

	// Create subdirectory path: baseDestDir/databaseName
	return baseDestDir + "/" + databaseName
}

// generateBackupName generates a backup name from template and database name
func (s *Service) generateBackupName(template, databaseName string) string {
	if template == "" {
		return databaseName + " Backup"
	}

	// Replace placeholders in template
	name := strings.ReplaceAll(template, "{database}", databaseName)
	name = strings.ReplaceAll(name, "{db}", databaseName)

	return name
}
