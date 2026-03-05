package main

import (
	"database/sql"
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

// closeDatabase closes the database connection
func closeDatabase() {
	if db != nil {
		db.Close()
		log.Println("INFO: Database connection closed")
	}
}
