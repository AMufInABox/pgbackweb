package postgres

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/eduardolat/pgbackweb/internal/util/strutil"
	"github.com/orsinium-labs/enum"
)

/*
	Important:
	Versions supported by PG Back Web must be supported in PostgreSQL Version Policy
	https://www.postgresql.org/support/versioning/

	Backing up a database from an old unsupported version should not be allowed.
*/

type version struct {
	Version string
	PGDump  string
	PSQL    string
}

type PGVersion enum.Member[version]

var (
	PG13 = PGVersion{version{
		Version: "13",
		PGDump:  "/usr/lib/postgresql/13/bin/pg_dump",
		PSQL:    "/usr/lib/postgresql/13/bin/psql",
	}}
	PG14 = PGVersion{version{
		Version: "14",
		PGDump:  "/usr/lib/postgresql/14/bin/pg_dump",
		PSQL:    "/usr/lib/postgresql/14/bin/psql",
	}}
	PG15 = PGVersion{version{
		Version: "15",
		PGDump:  "/usr/lib/postgresql/15/bin/pg_dump",
		PSQL:    "/usr/lib/postgresql/15/bin/psql",
	}}
	PG16 = PGVersion{version{
		Version: "16",
		PGDump:  "/usr/lib/postgresql/16/bin/pg_dump",
		PSQL:    "/usr/lib/postgresql/16/bin/psql",
	}}
	PG17 = PGVersion{version{
		Version: "17",
		PGDump:  "/usr/lib/postgresql/17/bin/pg_dump",
		PSQL:    "/usr/lib/postgresql/17/bin/psql",
	}}
	PG18 = PGVersion{version{
		Version: "18",
		PGDump:  "/usr/lib/postgresql/18/bin/pg_dump",
		PSQL:    "/usr/lib/postgresql/18/bin/psql",
	}}

	PGVersions     = []PGVersion{PG13, PG14, PG15, PG16, PG17, PG18}
	PGVersionsDesc = []PGVersion{PG18, PG17, PG16, PG15, PG14, PG13}
)

type Client struct{}

func New() *Client {
	return &Client{}
}

// ParseVersion returns the PGVersion enum member for the given PostgreSQL
// version as a string.
func (Client) ParseVersion(version string) (PGVersion, error) {
	switch version {
	case "13":
		return PG13, nil
	case "14":
		return PG14, nil
	case "15":
		return PG15, nil
	case "16":
		return PG16, nil
	case "17":
		return PG17, nil
	case "18":
		return PG18, nil
	default:
		return PGVersion{}, fmt.Errorf("pg version not allowed: %s", version)
	}
}

// Test tests the connection to the PostgreSQL database
func (Client) Test(version PGVersion, connString string) error {
	cmd := exec.Command(version.Value.PSQL, connString, "-c", "SELECT 1;")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"error running psql test v%s: %s",
			version.Value.Version, output,
		)
	}

	return nil
}

// DumpParams contains the parameters for the pg_dump command
type DumpParams struct {
	// DataOnly (--data-only): Dump only the data, not the schema (data definitions).
	// Table data, large objects, and sequence values are dumped.
	DataOnly bool

	// SchemaOnly (--schema-only): Dump only the object definitions (schema), not data.
	SchemaOnly bool

	// Clean (--clean): Output commands to DROP all the dumped database objects
	// prior to outputting the commands for creating them. This option is useful
	// when the restore is to overwrite an existing database. If any of the
	// objects do not exist in the destination database, ignorable error messages
	// will be reported during restore, unless --if-exists is also specified.
	Clean bool

	// IfExists (--if-exists): Use DROP ... IF EXISTS commands to drop objects in
	// --clean mode. This suppresses "does not exist" errors that might otherwise
	// be reported. This option is not valid unless --clean is also specified.
	IfExists bool

	// Create (--create): Begin the output with a command to create the database
	// itself and reconnect to the created database. (With a script of this form,
	// it doesn't matter which database in the destination installation you
	// connect to before running the script.) If --clean is also specified, the
	// script drops and recreates the target database before reconnecting to it.
	Create bool

	// NoComments (--no-comments): Do not dump comments.
	NoComments bool
}

// Dump runs the pg_dump command with the given parameters. It returns the SQL
// dump as an io.Reader.
func (Client) Dump(
	version PGVersion, connString string, params ...DumpParams,
) io.Reader {
	pickedParams := DumpParams{}
	if len(params) > 0 {
		pickedParams = params[0]
	}

	args := []string{connString, "--no-owner", "--no-acl"}
	if pickedParams.DataOnly {
		args = append(args, "--data-only")
	}
	if pickedParams.SchemaOnly {
		args = append(args, "--schema-only")
	}
	if pickedParams.Clean {
		args = append(args, "--clean")
	}
	if pickedParams.IfExists {
		args = append(args, "--if-exists")
	}
	if pickedParams.Create {
		args = append(args, "--create")
	}
	if pickedParams.NoComments {
		args = append(args, "--no-comments")
	}

	errorBuffer := &bytes.Buffer{}
	reader, writer := io.Pipe()
	cmd := exec.Command(version.Value.PGDump, args...)
	cmd.Stdout = writer
	cmd.Stderr = errorBuffer

	go func() {
		defer writer.Close()
		if err := cmd.Run(); err != nil {
			writer.CloseWithError(fmt.Errorf(
				"error running pg_dump v%s: %s",
				version.Value.Version, errorBuffer.String(),
			))
		}
	}()

	return reader
}

// DumpZip runs the pg_dump command with the given parameters and returns the
// ZIP-compressed SQL dump as an io.Reader.
func (c *Client) DumpZip(
	version PGVersion, connString string, params ...DumpParams,
) io.Reader {
	dumpReader := c.Dump(version, connString, params...)
	reader, writer := io.Pipe()

	go func() {
		defer writer.Close()

		zipWriter := zip.NewWriter(writer)
		defer zipWriter.Close()

		fileWriter, err := zipWriter.Create("dump.sql")
		if err != nil {
			writer.CloseWithError(fmt.Errorf("error creating zip file: %w", err))
			return
		}

		if _, err := io.Copy(fileWriter, dumpReader); err != nil {
			writer.CloseWithError(fmt.Errorf("error writing to zip file: %w", err))
			return
		}
	}()

	return reader
}

// parseDatabaseName extracts the database name from a PostgreSQL connection string.
// It supports both URL format (postgres://user:pass@host:port/dbname) and
// key-value format (host=localhost dbname=mydb user=postgres).
func (Client) parseDatabaseName(connString string) (string, error) {
	// Try URL format first
	if strings.HasPrefix(connString, "postgres://") || strings.HasPrefix(connString, "postgresql://") {
		u, err := url.Parse(connString)
		if err != nil {
			return "", fmt.Errorf("error parsing connection string URL: %w", err)
		}
		dbName := strings.TrimPrefix(u.Path, "/")
		if dbName == "" {
			return "", fmt.Errorf("no database name found in connection string")
		}
		return dbName, nil
	}

	// Try key-value format
	parts := strings.Fields(connString)
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 && kv[0] == "dbname" {
			return kv[1], nil
		}
	}

	return "", fmt.Errorf("no database name found in connection string")
}

// replaceDatabase replaces the database name in a connection string with a new database name.
// This is used to connect to the postgres maintenance database instead of the target database.
func (Client) replaceDatabase(connString, newDBName string) (string, error) {
	// Try URL format first
	if strings.HasPrefix(connString, "postgres://") || strings.HasPrefix(connString, "postgresql://") {
		u, err := url.Parse(connString)
		if err != nil {
			return "", fmt.Errorf("error parsing connection string URL: %w", err)
		}
		u.Path = "/" + newDBName
		return u.String(), nil
	}

	// Try key-value format
	parts := strings.Fields(connString)
	var newParts []string
	foundDBName := false
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 && kv[0] == "dbname" {
			newParts = append(newParts, fmt.Sprintf("dbname=%s", newDBName))
			foundDBName = true
		} else {
			newParts = append(newParts, part)
		}
	}

	if !foundDBName {
		return "", fmt.Errorf("no database name found in connection string")
	}

	return strings.Join(newParts, " "), nil
}

// terminateConnections terminates all active connections to a database except the current one.
// This is necessary before dropping or restoring a database.
func (c *Client) terminateConnections(version PGVersion, connString, targetDB string) error {
	// Create connection string to postgres maintenance database
	maintenanceConnString, err := c.replaceDatabase(connString, "postgres")
	if err != nil {
		return fmt.Errorf("error creating maintenance connection string: %w", err)
	}

	// SQL to terminate all connections to the target database
	sql := fmt.Sprintf(
		"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s' AND pid <> pg_backend_pid();",
		strings.ReplaceAll(targetDB, "'", "''"), // Escape single quotes
	)

	cmd := exec.Command(version.Value.PSQL, maintenanceConnString, "-c", sql)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"error terminating connections to database %s: %s",
			targetDB, output,
		)
	}

	return nil
}

// RestoreZip downloads or copies the ZIP from the given url or path, unzips it,
// and runs the psql command to restore the database.
//
// The ZIP file must contain a dump.sql file with the SQL dump to restore.
//
//   - version: PostgreSQL version to use for the restore
//   - connString: connection string to the database
//   - isLocal: whether the ZIP file is local or a URL
//   - zipURLOrPath: URL or path to the ZIP file
func (c *Client) RestoreZip(
	version PGVersion, connString string, isLocal bool, zipURLOrPath string,
) error {
	// Create temp work directory
	workDir, err := os.MkdirTemp("", "pbw-restore-*")
	if err != nil {
		return fmt.Errorf("error creating temp dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	zipPath := strutil.CreatePath(true, workDir, "dump.zip")
	dumpPath := strutil.CreatePath(true, workDir, "dump.sql")

	// Handle local ZIP
	if isLocal {
		cmd := exec.Command("cp", zipURLOrPath, zipPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("error copying ZIP file to temp dir: %s", output)
		}
	}

	// Handle remote ZIP
	if !isLocal {
		cmd := exec.Command("curl", "-L", "-o", zipPath, zipURLOrPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("error downloading ZIP file: %s", string(output))
		}
	}

	// Verify ZIP exists
	if _, err := os.Stat(zipPath); os.IsNotExist(err) {
		return fmt.Errorf("zip file not found: %s", zipPath)
	}

	// Unzip dump.sql
	cmd := exec.Command("unzip", "-o", zipPath, "dump.sql", "-d", workDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error unzipping ZIP file: %s", output)
	}

	if _, err := os.Stat(dumpPath); os.IsNotExist(err) {
		return fmt.Errorf("dump.sql file not found in ZIP file: %s", zipPath)
	}

	// Read dump content to check for DROP/CREATE DATABASE
	dumpContent, err := os.ReadFile(dumpPath)
	if err != nil {
		return fmt.Errorf("error reading dump.sql file: %w", err)
	}
	dumpStr := string(dumpContent)
	containsDropDatabase := strings.Contains(dumpStr, "DROP DATABASE")
	containsCreateDatabase := strings.Contains(dumpStr, "CREATE DATABASE")

	if containsDropDatabase || containsCreateDatabase {
		// Extract database name
		dbName, err := c.parseDatabaseName(connString)
		if err != nil {
			return fmt.Errorf("error parsing database name from connection string: %w", err)
		}

		// Terminate active connections
		if err := c.terminateConnections(version, connString, dbName); err != nil {
			return fmt.Errorf("error terminating database connections: %w", err)
		}

		// Switch to maintenance DB
		maintenanceConnString, err := c.replaceDatabase(connString, "postgres")
		if err != nil {
			return fmt.Errorf("error creating maintenance connection string: %w", err)
		}
		connString = maintenanceConnString
	}

	// Execute restore
	cmd = exec.Command(version.Value.PSQL, connString, "-f", dumpPath)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running psql v%s command: %s", version.Value.Version, output)
	}

	return nil
}