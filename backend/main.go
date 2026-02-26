package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// ============================================================================
// Network PC Monitoring System - Backend Server
// ============================================================================
// This service provides REST APIs for monitoring the status of computers
// on a network via ping (TCP connection on port 22).
// ============================================================================

// Computer represents a network computer in the monitoring system
type Computer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	IP   string `json:"ip"`
}

// ComputerStatus contains the current status of a computer after a ping check
type ComputerStatus struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IP        string `json:"ip"`
	Status    string `json:"status"`      // "ON" or "OFF"
	CheckedAt string `json:"checkedAt"`   // Timestamp of last check
}

// APIResponse is the standard response format for all API endpoints
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ============================================================================
// Global State
// ============================================================================

// computers is the in-memory store of computers being monitored
// In production, this should be persisted to a database
var computers = []Computer{
	{ID: "1", Name: "Cabin 12", IP: "192.168.68.92"},
}

// ============================================================================
// Network Utilities
// ============================================================================

// pingHost attempts to establish a TCP connection to a host on port 22 (SSH)
// Returns "ON" if successful, "OFF" if connection fails
func pingHost(ip string) string {
	timeout := 2 * time.Second
	conn, err := net.DialTimeout("tcp", ip+":22", timeout)
	if err != nil {
		return "OFF"
	}
	defer conn.Close()
	return "ON"
}

// ============================================================================
// HTTP Utilities
// ============================================================================

// setCORS configures CORS headers for cross-origin requests
func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")
}

// writeJSON writes a JSON response with the specified HTTP status code
func writeJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("ERROR: Failed to encode JSON response: %v", err)
	}
}

// ============================================================================
// API Endpoints
// ============================================================================

// listComputers handles GET /api/computers
// Returns all computers in the monitoring system
func listComputers(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    computers,
	})
	log.Println("INFO: Listed all computers")
}

// addComputer handles POST /api/computers
// Adds a new computer to the monitoring system with validation
func addComputer(w http.ResponseWriter, r *http.Request) {
	setCORS(w)

	// Parse request body
	var c Computer
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid JSON body",
		})
		log.Printf("ERROR: Failed to decode computer data: %v", err)
		return
	}

	// Validate required fields
	if c.ID == "" || c.Name == "" || c.IP == "" {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "id, name and ip are required",
		})
		log.Println("WARNING: Attempt to add computer with missing fields")
		return
	}

	// Check for duplicate ID
	for _, existing := range computers {
		if existing.ID == c.ID {
			writeJSON(w, http.StatusConflict, APIResponse{
				Success: false,
				Error:   "Computer with this ID already exists",
			})
			log.Printf("WARNING: Attempt to add duplicate computer ID: %s", c.ID)
			return
		}
	}

	// Add the new computer
	computers = append(computers, c)
	writeJSON(w, http.StatusCreated, APIResponse{
		Success: true,
		Data:    c,
	})
	log.Printf("INFO: Computer added - ID: %s, Name: %s, IP: %s", c.ID, c.Name, c.IP)
}

// pingOne handles GET /api/ping/:id
// Pings a specific computer and returns its status
func pingOne(w http.ResponseWriter, r *http.Request) {
	setCORS(w)

	// Extract computer ID from URL path
	id := strings.TrimPrefix(r.URL.Path, "/api/ping/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Missing computer ID",
		})
		return
	}

	// Find and ping the computer
	for _, c := range computers {
		if c.ID == id {
			status := pingHost(c.IP)
			result := ComputerStatus{
				ID:        c.ID,
				Name:      c.Name,
				IP:        c.IP,
				Status:    status,
				CheckedAt: time.Now().Format("2006-01-02 15:04:05"),
			}
			writeJSON(w, http.StatusOK, APIResponse{
				Success: true,
				Data:    result,
			})
			log.Printf("INFO: Pinged computer %s (%s) - Status: %s", c.Name, c.IP, status)
			return
		}
	}

	// Computer not found
	writeJSON(w, http.StatusNotFound, APIResponse{
		Success: false,
		Error:   "Computer not found",
	})
	log.Printf("WARNING: Attempt to ping non-existent computer ID: %s", id)
}

// pingAll handles GET /api/ping-all
// Pings all computers and returns their statuses
func pingAll(w http.ResponseWriter, r *http.Request) {
	setCORS(w)

	now := time.Now().Format("2006-01-02 15:04:05")
	var results []ComputerStatus

	// Ping all computers concurrently
	for _, c := range computers {
		results = append(results, ComputerStatus{
			ID:        c.ID,
			Name:      c.Name,
			IP:        c.IP,
			Status:    pingHost(c.IP),
			CheckedAt: now,
		})
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    results,
	})

	// Log summary
	onCount := 0
	for _, r := range results {
		if r.Status == "ON" {
			onCount++
		}
	}
	log.Printf("INFO: Pinged all %d computers - Online: %d, Offline: %d", len(computers), onCount, len(computers)-onCount)
}

// ============================================================================
// Router
// ============================================================================

// router is the main HTTP request handler that routes requests to appropriate endpoints
func router(w http.ResponseWriter, r *http.Request) {
	// Handle CORS preflight requests
	if r.Method == http.MethodOptions {
		setCORS(w)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	path := r.URL.Path

	// Route to appropriate handler
	switch {
	case path == "/api/computers" && r.Method == http.MethodGet:
		listComputers(w, r)
	case path == "/api/computers" && r.Method == http.MethodPost:
		addComputer(w, r)
	case strings.HasPrefix(path, "/api/ping/") && r.Method == http.MethodGet:
		pingOne(w, r)
	case path == "/api/ping-all" && r.Method == http.MethodGet:
		pingAll(w, r)
	default:
		// Serve frontend static files
		http.FileServer(http.Dir("../frontend")).ServeHTTP(w, r)
	}
}

// ============================================================================
// Main
// ============================================================================

func main() {
	// Register router
	http.HandleFunc("/", router)

	// Server configuration
	port := ":8081"
	log.Printf("========================================")
	log.Printf("Network PC Monitoring System - Started")
	log.Printf("Server running at http://localhost%s", port)
	log.Printf("========================================")

	// Start server
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("FATAL: Server failed to start: %v", err)
	}
}
