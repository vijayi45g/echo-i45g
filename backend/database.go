package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// ============================================================================
// Database Manager
// ============================================================================

var db *sql.DB

// initDatabase initializes the SQLite database connection and creates tables
func initDatabase(dbPath string) error {
	var err error
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("FATAL: Failed to open database: %v", err)
		return err
	}

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatalf("FATAL: Failed to connect to database: %v", err)
		return err
	}

	log.Printf("INFO: Database connected successfully at %s", dbPath)

	// Create tables if they don't exist
	if err := createTables(); err != nil {
		log.Fatalf("FATAL: Failed to create tables: %v", err)
		return err
	}

	return nil
}

// createTables creates the necessary database tables
func createTables() error {
	schema := `
	CREATE TABLE IF NOT EXISTS computers (
		id TEXT PRIMARY KEY,
		place TEXT NOT NULL,
		username TEXT NOT NULL DEFAULT 'root',
		ip TEXT NOT NULL UNIQUE,
		os_name TEXT NOT NULL DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS merge_history (
		id               INTEGER PRIMARY KEY AUTOINCREMENT,
		source_machine_id TEXT NOT NULL,
		source_folder     TEXT NOT NULL,
		target_machine_id TEXT NOT NULL,
		target_folder     TEXT NOT NULL,
		backup_path       TEXT NOT NULL,
		files_merged      TEXT NOT NULL DEFAULT '[]',
		files_conflicted  TEXT NOT NULL DEFAULT '[]',
		status            TEXT NOT NULL DEFAULT 'completed',
		created_at        DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS merge_file_hashes (
		id                INTEGER PRIMARY KEY AUTOINCREMENT,
		target_machine_id TEXT NOT NULL,
		target_folder     TEXT NOT NULL,
		relative_path     TEXT NOT NULL,
		sha256_hash       TEXT NOT NULL,
		updated_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(target_machine_id, target_folder, relative_path)
	);


	CREATE TABLE IF NOT EXISTS health_status (
		computer_id  TEXT PRIMARY KEY,
		status       TEXT NOT NULL DEFAULT 'UNKNOWN',
		os_name      TEXT NOT NULL DEFAULT '',
		last_checked DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (computer_id) REFERENCES computers(id) ON DELETE CASCADE
	);
	`

	_, err := db.Exec(schema)
	if err != nil {
		log.Printf("ERROR: Failed to create tables: %v", err)
		return err
	}

	log.Println("INFO: Database tables initialized")

	// Schema migration for existing databases created before os_name existed.
	if _, err := db.Exec("ALTER TABLE computers ADD COLUMN os_name TEXT NOT NULL DEFAULT ''"); err != nil {
		if !strings.Contains(err.Error(), "duplicate column name") {
			log.Printf("ERROR: Failed to add os_name column: %v", err)
			return err
		}
	}

	return nil
}

// ============================================================================
// Smart Merge Database Operations
// ============================================================================

// insertMergeHistory records a completed merge operation and returns the new row ID.
func insertMergeHistory(sourceMachineID, sourceFolder, targetMachineID, targetFolder, backupPath string, filesMerged, filesConflicted []string) (int64, error) {
	mergedJSON, _ := json.Marshal(filesMerged)
	conflictedJSON, _ := json.Marshal(filesConflicted)

	result, err := db.Exec(
		`INSERT INTO merge_history (source_machine_id, source_folder, target_machine_id, target_folder, backup_path, files_merged, files_conflicted)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		sourceMachineID, sourceFolder, targetMachineID, targetFolder, backupPath, string(mergedJSON), string(conflictedJSON),
	)
	if err != nil {
		log.Printf("ERROR: Failed to insert merge history: %v", err)
		return 0, err
	}
	return result.LastInsertId()
}

// MergeHistoryRow represents one row from the merge_history table.
type MergeHistoryRow struct {
	ID              int      `json:"id"`
	SourceMachineID string   `json:"sourceMachineId"`
	SourceFolder    string   `json:"sourceFolder"`
	TargetMachineID string   `json:"targetMachineId"`
	TargetFolder    string   `json:"targetFolder"`
	BackupPath      string   `json:"backupPath"`
	FilesMerged     []string `json:"filesMerged"`
	FilesConflicted []string `json:"filesConflicted"`
	Status          string   `json:"status"`
	CreatedAt       string   `json:"createdAt"`
}

// getMergeHistory returns the last `limit` merge records for the given target.
// When targetFolder is empty, returns history across all folders for that computer.
func getMergeHistory(targetMachineID, targetFolder string, limit int) ([]MergeHistoryRow, error) {
	var rows *sql.Rows
	var err error

	if targetFolder != "" {
		rows, err = db.Query(
			`SELECT id, source_machine_id, source_folder, target_machine_id, target_folder,
			        backup_path, files_merged, files_conflicted, status, created_at
			 FROM merge_history
			 WHERE target_machine_id = ? AND target_folder = ?
			 ORDER BY created_at DESC
			 LIMIT ?`,
			targetMachineID, targetFolder, limit,
		)
	} else {
		rows, err = db.Query(
			`SELECT id, source_machine_id, source_folder, target_machine_id, target_folder,
			        backup_path, files_merged, files_conflicted, status, created_at
			 FROM merge_history
			 WHERE target_machine_id = ?
			 ORDER BY created_at DESC
			 LIMIT ?`,
			targetMachineID, limit,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []MergeHistoryRow
	for rows.Next() {
		var r MergeHistoryRow
		var mergedJSON, conflictedJSON string
		if err := rows.Scan(&r.ID, &r.SourceMachineID, &r.SourceFolder, &r.TargetMachineID,
			&r.TargetFolder, &r.BackupPath, &mergedJSON, &conflictedJSON, &r.Status, &r.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(mergedJSON), &r.FilesMerged)
		_ = json.Unmarshal([]byte(conflictedJSON), &r.FilesConflicted)
		if r.FilesMerged == nil {
			r.FilesMerged = []string{}
		}
		if r.FilesConflicted == nil {
			r.FilesConflicted = []string{}
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// getMergeHistoryByID retrieves a single merge record by its primary key.
func getMergeHistoryByID(mergeID int) (*MergeHistoryRow, error) {
	var r MergeHistoryRow
	var mergedJSON, conflictedJSON string
	err := db.QueryRow(
		`SELECT id, source_machine_id, source_folder, target_machine_id, target_folder,
		        backup_path, files_merged, files_conflicted, status, created_at
		 FROM merge_history WHERE id = ?`, mergeID,
	).Scan(&r.ID, &r.SourceMachineID, &r.SourceFolder, &r.TargetMachineID,
		&r.TargetFolder, &r.BackupPath, &mergedJSON, &conflictedJSON, &r.Status, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(mergedJSON), &r.FilesMerged)
	_ = json.Unmarshal([]byte(conflictedJSON), &r.FilesConflicted)
	if r.FilesMerged == nil {
		r.FilesMerged = []string{}
	}
	if r.FilesConflicted == nil {
		r.FilesConflicted = []string{}
	}
	return &r, nil
}

// markMergeRolledBack sets the status of a merge to "rolled_back".
func markMergeRolledBack(mergeID int) error {
	_, err := db.Exec(`UPDATE merge_history SET status = 'rolled_back' WHERE id = ?`, mergeID)
	return err
}

// upsertFileHash records or updates the SHA-256 hash for a file after a successful merge.
func upsertFileHash(targetMachineID, targetFolder, relativePath, hash string) error {
	_, err := db.Exec(
		`INSERT INTO merge_file_hashes (target_machine_id, target_folder, relative_path, sha256_hash, updated_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(target_machine_id, target_folder, relative_path)
		 DO UPDATE SET sha256_hash = excluded.sha256_hash, updated_at = CURRENT_TIMESTAMP`,
		targetMachineID, targetFolder, relativePath, hash,
	)
	return err
}

// getFileHash returns the last-known SHA-256 for a given file path on a target, or "" if not found.
func getFileHash(targetMachineID, targetFolder, relativePath string) string {
	var hash string
	err := db.QueryRow(
		`SELECT sha256_hash FROM merge_file_hashes
		 WHERE target_machine_id = ? AND target_folder = ? AND relative_path = ?`,
		targetMachineID, targetFolder, relativePath,
	).Scan(&hash)
	if err != nil {
		return ""
	}
	return hash
}

// ============================================================================
// Database Operations
// ============================================================================

// getComputers retrieves all computers from the database
func getComputers() ([]Computer, error) {
	rows, err := db.Query("SELECT id, place, username, ip, os_name FROM computers ORDER BY created_at DESC")
	if err != nil {
		log.Printf("ERROR: Failed to query computers: %v", err)
		return nil, err
	}
	defer rows.Close()

	var computers []Computer
	for rows.Next() {
		var c Computer
		if err := rows.Scan(&c.ID, &c.Place, &c.Username, &c.IP, &c.OS); err != nil {
			log.Printf("ERROR: Failed to scan computer row: %v", err)
			return nil, err
		}
		computers = append(computers, c)
	}

	if err := rows.Err(); err != nil {
		log.Printf("ERROR: Error iterating rows: %v", err)
		return nil, err
	}

	return computers, nil
}

// createComputer inserts a new computer into the database
func createComputer(c Computer) error {
	_, err := db.Exec(
		"INSERT INTO computers (id, place, username, ip, os_name) VALUES (?, ?, ?, ?, ?)",
		c.ID, c.Place, c.Username, c.IP, c.OS,
	)
	if err != nil {
		log.Printf("ERROR: Failed to insert computer: %v", err)
		return err
	}

	log.Printf("INFO: Computer saved to database - ID: %s, Place: %s, Username: %s, IP: %s", c.ID, c.Place, c.Username, c.IP)
	return nil
}

// computerExists checks if a computer with given ID exists
func computerExists(id string) bool {
	var existingID string
	err := db.QueryRow("SELECT id FROM computers WHERE id = ?", id).Scan(&existingID)
	return err == nil
}

// updateComputer updates an existing computer in the database
func updateComputer(c Computer) error {
	result, err := db.Exec(
		"UPDATE computers SET place = ?, username = ?, ip = ? WHERE id = ?",
		c.Place, c.Username, c.IP, c.ID,
	)
	if err != nil {
		log.Printf("ERROR: Failed to update computer: %v", err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("ERROR: Failed to get rows affected: %v", err)
		return err
	}

	if rowsAffected == 0 {
		log.Printf("WARNING: Computer not found for update - ID: %s", c.ID)
		return sql.ErrNoRows
	}

	log.Printf("INFO: Computer updated - ID: %s, Place: %s, Username: %s, IP: %s", c.ID, c.Place, c.Username, c.IP)
	return nil
}

// updateComputerOS updates only os_name for an existing computer.
func updateComputerOS(id, osName string) error {
	result, err := db.Exec("UPDATE computers SET os_name = ? WHERE id = ?", osName, id)
	if err != nil {
		log.Printf("ERROR: Failed to update OS for ID %s: %v", id, err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("ERROR: Failed to get rows affected for OS update: %v", err)
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	log.Printf("INFO: Computer OS updated - ID: %s, OS: %s", id, osName)
	return nil
}

// deleteComputer removes a computer from the database
func deleteComputer(id string) error {
	result, err := db.Exec("DELETE FROM computers WHERE id = ?", id)
	if err != nil {
		log.Printf("ERROR: Failed to delete computer: %v", err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("ERROR: Failed to get rows affected: %v", err)
		return err
	}

	if rowsAffected == 0 {
		log.Printf("WARNING: Computer not found for deletion - ID: %s", id)
		return sql.ErrNoRows
	}

	log.Printf("INFO: Computer deleted - ID: %s", id)
	return nil
}

// ============================================================================
// Health Status Database Operations
// ============================================================================

// HealthStatusRow represents a single health check result.
type HealthStatusRow struct {
	ComputerID  string `json:"computerId"`
	Status      string `json:"status"`
	OSName      string `json:"osName"`
	LastChecked string `json:"lastChecked"`
}

// upsertHealthStatus stores or updates the health status for a computer.
func upsertHealthStatus(computerID, status, osName string) error {
	_, err := db.Exec(
		`INSERT INTO health_status (computer_id, status, os_name, last_checked)
		 VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(computer_id)
		 DO UPDATE SET status = excluded.status, os_name = excluded.os_name, last_checked = CURRENT_TIMESTAMP`,
		computerID, status, osName,
	)
	if err != nil {
		log.Printf("ERROR: Failed to upsert health status for %s: %v", computerID, err)
	}
	return err
}

// getHealthStatuses returns all health statuses.
func getHealthStatuses() ([]HealthStatusRow, error) {
	rows, err := db.Query(
		`SELECT computer_id, status, os_name, last_checked FROM health_status ORDER BY computer_id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []HealthStatusRow
	for rows.Next() {
		var r HealthStatusRow
		if err := rows.Scan(&r.ComputerID, &r.Status, &r.OSName, &r.LastChecked); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// getHealthStatus returns a single health status, or nil if not found.
func getHealthStatus(computerID string) (*HealthStatusRow, error) {
	var r HealthStatusRow
	err := db.QueryRow(
		`SELECT computer_id, status, os_name, last_checked FROM health_status WHERE computer_id = ?`,
		computerID,
	).Scan(&r.ComputerID, &r.Status, &r.OSName, &r.LastChecked)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// deleteHealthStatus removes the health record when a computer is deleted.
func deleteHealthStatus(computerID string) {
	_, _ = db.Exec(`DELETE FROM health_status WHERE computer_id = ?`, computerID)
}

// closeDatabase closes the database connection
func closeDatabase() {
	if db != nil {
		db.Close()
		log.Println("INFO: Database connection closed")
	}
}
