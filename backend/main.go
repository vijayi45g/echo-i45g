package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

// ============================================================================
// Network PC Monitoring System - Backend Server
// ============================================================================
// This service provides REST APIs for monitoring the status of computers
// on a network via ping (TCP connection on port 22).
// ============================================================================

// Computer represents a network computer in the monitoring system
type Computer struct {
	ID       string `json:"id"`
	Place    string `json:"place"`
	Username string `json:"username"`
	IP       string `json:"ip"`
	OS       string `json:"os"`
}

// ComputerStatus contains the current status of a computer after a ping check
type ComputerStatus struct {
	ID        string `json:"id"`
	Place     string `json:"place"`
	Username  string `json:"username"`
	IP        string `json:"ip"`
	Status    string `json:"status"`    // "ON" or "OFF"
	CheckedAt string `json:"checkedAt"` // Timestamp of last check
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

// Database path - stores SQLite database file
const dbPath = "monitoring.db"

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
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
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

	computers, err := getComputers()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Failed to retrieve computers",
		})
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    computers,
	})
	log.Println("INFO: Listed all computers from database")
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
	if c.ID == "" || c.Place == "" || c.Username == "" || c.IP == "" {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "id, place, username and ip are required",
		})
		log.Println("WARNING: Attempt to add computer with missing fields")
		return
	}

	// Check for duplicate ID
	if computerExists(c.ID) {
		writeJSON(w, http.StatusConflict, APIResponse{
			Success: false,
			Error:   "Computer with this ID already exists",
		})
		log.Printf("WARNING: Attempt to add duplicate computer ID: %s", c.ID)
		return
	}

	// Add the new computer to database
	if err := createComputer(c); err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Failed to save computer to database",
		})
		return
	}

	writeJSON(w, http.StatusCreated, APIResponse{
		Success: true,
		Data:    c,
	})
	log.Printf("INFO: Computer added to database - ID: %s, Place: %s, Username: %s, IP: %s", c.ID, c.Place, c.Username, c.IP)
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

	// Get computers from database
	computers, err := getComputers()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Failed to retrieve computers",
		})
		return
	}

	// Find and ping the computer
	for _, c := range computers {
		if c.ID == id {
			status := pingHost(c.IP)
			result := ComputerStatus{
				ID:        c.ID,
				Place:     c.Place,
				Username:  c.Username,
				IP:        c.IP,
				Status:    status,
				CheckedAt: time.Now().Format("2006-01-02 15:04:05"),
			}
			writeJSON(w, http.StatusOK, APIResponse{
				Success: true,
				Data:    result,
			})
			log.Printf("INFO: Pinged computer %s (%s) - Status: %s", c.Place, c.IP, status)
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

	// Get all computers from database
	computers, err := getComputers()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Failed to retrieve computers",
		})
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	var results []ComputerStatus

	// Ping all computers
	for _, c := range computers {
		results = append(results, ComputerStatus{
			ID:        c.ID,
			Place:     c.Place,
			Username:  c.Username,
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

// editComputer handles PUT /api/computers/:id
// Updates an existing computer's information
func editComputer(w http.ResponseWriter, r *http.Request) {
	setCORS(w)

	// Extract computer ID from URL path
	id := strings.TrimPrefix(r.URL.Path, "/api/computers/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Missing computer ID",
		})
		return
	}

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
	if c.Place == "" || c.Username == "" || c.IP == "" {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "place, username and ip are required",
		})
		return
	}

	// Set the ID and update
	c.ID = id
	if err := updateComputer(c); err != nil {
		writeJSON(w, http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "Computer not found",
		})
		log.Printf("WARNING: Attempt to update non-existent computer ID: %s", id)
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    c,
	})
	log.Printf("INFO: Computer updated - ID: %s, Place: %s, Username: %s, IP: %s", c.ID, c.Place, c.Username, c.IP)
}

// deleteComputerHandler handles DELETE /api/computers/:id
// Deletes a computer from the monitoring system
func deleteComputerHandler(w http.ResponseWriter, r *http.Request) {
	setCORS(w)

	// Extract computer ID from URL path
	id := strings.TrimPrefix(r.URL.Path, "/api/computers/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Missing computer ID",
		})
		return
	}

	// Delete the computer
	if err := deleteComputer(id); err != nil {
		writeJSON(w, http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "Computer not found",
		})
		log.Printf("WARNING: Attempt to delete non-existent computer ID: %s", id)
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    map[string]string{"id": id},
	})
	log.Printf("INFO: Computer deleted - ID: %s", id)
}

// ============================================================================
// SSH Terminal Execution
// ============================================================================

// TerminalRequest represents a request to execute a command via SSH
type TerminalRequest struct {
	ComputerID string `json:"computerId"`
	Command    string `json:"command"`
	Username   string `json:"username,omitempty"`
}

// TerminalResponse represents the response from SSH command execution
type TerminalResponse struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// SystemInfo contains host OS + memory/disk metrics from a remote Linux machine.
type SystemInfo struct {
	ComputerID         string  `json:"computerId"`
	Place              string  `json:"place"`
	Username           string  `json:"username"`
	IP                 string  `json:"ip"`
	OS                 string  `json:"os"`
	MemoryTotalGB      float64 `json:"memoryTotalGB"`
	MemoryUsedGB       float64 `json:"memoryUsedGB"`
	MemoryUsagePercent float64 `json:"memoryUsagePercent"`
	DiskTotalGB        float64 `json:"diskTotalGB"`
	DiskUsedGB         float64 `json:"diskUsedGB"`
	DiskUsagePercent   float64 `json:"diskUsagePercent"`
	CollectedAt        string  `json:"collectedAt"`
}

type CPUCommandOutput struct {
	Free  string `json:"free"`
	LSCPU string `json:"lscpu"`
	Dmesg string `json:"dmesg"`
}

type CPUOverview struct {
	ComputerID         string           `json:"computerId"`
	Place              string           `json:"place"`
	Username           string           `json:"username"`
	IP                 string           `json:"ip"`
	OS                 string           `json:"os"`
	Kernel             string           `json:"kernel"`
	Uptime             string           `json:"uptime"`
	Architecture       string           `json:"architecture"`
	CPUModel           string           `json:"cpuModel"`
	CoreCount          int              `json:"coreCount"`
	ThreadsPerCore     int              `json:"threadsPerCore"`
	SocketCount        int              `json:"socketCount"`
	Load1              float64          `json:"load1"`
	Load5              float64          `json:"load5"`
	Load15             float64          `json:"load15"`
	CPUUsagePercent    float64          `json:"cpuUsagePercent"`
	MemoryTotalGB      float64          `json:"memoryTotalGB"`
	MemoryUsedGB       float64          `json:"memoryUsedGB"`
	MemoryUsagePercent float64          `json:"memoryUsagePercent"`
	DiskTotalGB        float64          `json:"diskTotalGB"`
	DiskUsedGB         float64          `json:"diskUsedGB"`
	DiskUsagePercent   float64          `json:"diskUsagePercent"`
	Commands           CPUCommandOutput `json:"commands"`
	CollectedAt        string           `json:"collectedAt"`
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func parseKeyValueOutput(output string) map[string]string {
	values := make(map[string]string)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		values[key] = value
	}
	return values
}

func parseFloatOrZero(value string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0
	}
	return v
}

func parseIntOrZero(value string) int {
	v, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return v
}

func truncateText(value string, maxLen int) string {
	if len(value) <= maxLen {
		return value
	}
	return value[:maxLen] + "\n...truncated..."
}

func collectSystemInfo(computer Computer) (SystemInfo, error) {
	// Linux-focused script (Debian/Ubuntu compatible).
	// Emits KEY=VALUE lines for predictable parsing.
	script := `
OS=$(awk -F= '/^PRETTY_NAME=/{gsub(/"/, "", $2); print $2}' /etc/os-release 2>/dev/null)
if [ -z "$OS" ]; then OS=$(uname -s); fi

MEM_TOTAL_KB=$(awk '/MemTotal/ {print $2}' /proc/meminfo 2>/dev/null)
MEM_AVAIL_KB=$(awk '/MemAvailable/ {print $2}' /proc/meminfo 2>/dev/null)
if [ -z "$MEM_AVAIL_KB" ]; then MEM_AVAIL_KB=$(awk '/MemFree/ {print $2}' /proc/meminfo 2>/dev/null); fi
if [ -z "$MEM_TOTAL_KB" ]; then MEM_TOTAL_KB=0; fi
if [ -z "$MEM_AVAIL_KB" ]; then MEM_AVAIL_KB=0; fi
MEM_USED_KB=$((MEM_TOTAL_KB - MEM_AVAIL_KB))
if [ "$MEM_USED_KB" -lt 0 ]; then MEM_USED_KB=0; fi

DISK_TOTAL_KB=$(df -kP / 2>/dev/null | awk 'NR==2 {print $2}')
DISK_USED_KB=$(df -kP / 2>/dev/null | awk 'NR==2 {print $3}')
DISK_USED_PCT=$(df -kP / 2>/dev/null | awk 'NR==2 {gsub(/%/, "", $5); print $5}')
if [ -z "$DISK_TOTAL_KB" ]; then DISK_TOTAL_KB=0; fi
if [ -z "$DISK_USED_KB" ]; then DISK_USED_KB=0; fi
if [ -z "$DISK_USED_PCT" ]; then DISK_USED_PCT=0; fi

echo "OS=$OS"
echo "MEM_TOTAL_KB=$MEM_TOTAL_KB"
echo "MEM_USED_KB=$MEM_USED_KB"
echo "DISK_TOTAL_KB=$DISK_TOTAL_KB"
echo "DISK_USED_KB=$DISK_USED_KB"
echo "DISK_USED_PCT=$DISK_USED_PCT"
`

	command := fmt.Sprintf("bash -lc %q", script)
	output, err := executeSSHCommand(computer.IP, computer.Username, command)
	if err != nil {
		return SystemInfo{}, err
	}

	values := parseKeyValueOutput(output)
	if len(values) == 0 {
		return SystemInfo{}, errors.New("unable to parse system metrics from SSH output")
	}

	memoryTotalKB := parseFloatOrZero(values["MEM_TOTAL_KB"])
	memoryUsedKB := parseFloatOrZero(values["MEM_USED_KB"])
	diskTotalKB := parseFloatOrZero(values["DISK_TOTAL_KB"])
	diskUsedKB := parseFloatOrZero(values["DISK_USED_KB"])
	diskUsagePercent := parseFloatOrZero(values["DISK_USED_PCT"])

	if memoryUsedKB > memoryTotalKB && memoryTotalKB > 0 {
		memoryUsedKB = memoryTotalKB
	}
	if diskUsedKB > diskTotalKB && diskTotalKB > 0 {
		diskUsedKB = diskTotalKB
	}

	memoryUsagePercent := 0.0
	if memoryTotalKB > 0 {
		memoryUsagePercent = (memoryUsedKB / memoryTotalKB) * 100
	}
	if diskUsagePercent == 0 && diskTotalKB > 0 {
		diskUsagePercent = (diskUsedKB / diskTotalKB) * 100
	}

	const kbPerGB = 1024.0 * 1024.0
	osName := values["OS"]
	if osName == "" {
		osName = "Unknown Linux"
	}

	return SystemInfo{
		ComputerID:         computer.ID,
		Place:              computer.Place,
		Username:           computer.Username,
		IP:                 computer.IP,
		OS:                 osName,
		MemoryTotalGB:      round2(memoryTotalKB / kbPerGB),
		MemoryUsedGB:       round2(memoryUsedKB / kbPerGB),
		MemoryUsagePercent: round2(memoryUsagePercent),
		DiskTotalGB:        round2(diskTotalKB / kbPerGB),
		DiskUsedGB:         round2(diskUsedKB / kbPerGB),
		DiskUsagePercent:   round2(diskUsagePercent),
		CollectedAt:        time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

func collectCPUOverview(computer Computer) (CPUOverview, error) {
	systemInfo, err := collectSystemInfo(computer)
	if err != nil {
		return CPUOverview{}, err
	}

	summaryScript := `
CPU_MODEL=$(lscpu 2>/dev/null | awk -F: '/Model name/ {sub(/^[ \t]+/, "", $2); print $2; exit}')
ARCH=$(lscpu 2>/dev/null | awk -F: '/Architecture/ {sub(/^[ \t]+/, "", $2); print $2; exit}')
THREADS_PER_CORE=$(lscpu 2>/dev/null | awk -F: '/Thread\(s\) per core/ {sub(/^[ \t]+/, "", $2); print $2; exit}')
SOCKETS=$(lscpu 2>/dev/null | awk -F: '/Socket\(s\)/ {sub(/^[ \t]+/, "", $2); print $2; exit}')
CORES=$(nproc --all 2>/dev/null)
if [ -z "$CORES" ]; then CORES=$(lscpu 2>/dev/null | awk -F: '/^CPU\(s\):/ {sub(/^[ \t]+/, "", $2); print $2; exit}'); fi

LOAD1=$(awk '{print $1}' /proc/loadavg 2>/dev/null)
LOAD5=$(awk '{print $2}' /proc/loadavg 2>/dev/null)
LOAD15=$(awk '{print $3}' /proc/loadavg 2>/dev/null)

KERNEL=$(uname -r 2>/dev/null)
UPTIME=$(uptime -p 2>/dev/null)
if [ -z "$UPTIME" ]; then UPTIME=$(uptime 2>/dev/null); fi

echo "CPU_MODEL=$CPU_MODEL"
echo "ARCH=$ARCH"
echo "THREADS_PER_CORE=${THREADS_PER_CORE:-0}"
echo "SOCKETS=${SOCKETS:-0}"
echo "CORES=${CORES:-0}"
echo "LOAD1=${LOAD1:-0}"
echo "LOAD5=${LOAD5:-0}"
echo "LOAD15=${LOAD15:-0}"
echo "KERNEL=$KERNEL"
echo "UPTIME=$UPTIME"
`

	summaryCommand := fmt.Sprintf("bash -lc %q", summaryScript)
	summaryOut, err := executeSSHCommand(computer.IP, computer.Username, summaryCommand)
	if err != nil {
		return CPUOverview{}, err
	}

	summary := parseKeyValueOutput(summaryOut)
	if len(summary) == 0 {
		return CPUOverview{}, errors.New("unable to parse cpu overview metrics from SSH output")
	}

	coreCount := parseIntOrZero(summary["CORES"])
	if coreCount <= 0 {
		coreCount = 1
	}

	load1 := parseFloatOrZero(summary["LOAD1"])
	load5 := parseFloatOrZero(summary["LOAD5"])
	load15 := parseFloatOrZero(summary["LOAD15"])
	cpuUsagePercent := 0.0
	if coreCount > 0 {
		cpuUsagePercent = (load1 / float64(coreCount)) * 100
	}
	cpuUsagePercent = clamp(cpuUsagePercent, 0, 100)

	freeOut, _ := executeSSHCommand(computer.IP, computer.Username, "free -h 2>&1")
	lscpuOut, _ := executeSSHCommand(computer.IP, computer.Username, "lscpu 2>&1")
	dmesgOut, _ := executeSSHCommand(computer.IP, computer.Username, "dmesg 2>&1 | tail -n 25")

	if strings.TrimSpace(freeOut) == "" {
		freeOut = "No output from free -h"
	}
	if strings.TrimSpace(lscpuOut) == "" {
		lscpuOut = "No output from lscpu"
	}
	if strings.TrimSpace(dmesgOut) == "" {
		dmesgOut = "No output from dmesg"
	}

	return CPUOverview{
		ComputerID:         computer.ID,
		Place:              computer.Place,
		Username:           computer.Username,
		IP:                 computer.IP,
		OS:                 systemInfo.OS,
		Kernel:             summary["KERNEL"],
		Uptime:             summary["UPTIME"],
		Architecture:       summary["ARCH"],
		CPUModel:           summary["CPU_MODEL"],
		CoreCount:          coreCount,
		ThreadsPerCore:     parseIntOrZero(summary["THREADS_PER_CORE"]),
		SocketCount:        parseIntOrZero(summary["SOCKETS"]),
		Load1:              round2(load1),
		Load5:              round2(load5),
		Load15:             round2(load15),
		CPUUsagePercent:    round2(cpuUsagePercent),
		MemoryTotalGB:      systemInfo.MemoryTotalGB,
		MemoryUsedGB:       systemInfo.MemoryUsedGB,
		MemoryUsagePercent: systemInfo.MemoryUsagePercent,
		DiskTotalGB:        systemInfo.DiskTotalGB,
		DiskUsedGB:         systemInfo.DiskUsedGB,
		DiskUsagePercent:   systemInfo.DiskUsagePercent,
		Commands: CPUCommandOutput{
			Free:  truncateText(freeOut, 6000),
			LSCPU: truncateText(lscpuOut, 6000),
			Dmesg: truncateText(dmesgOut, 6000),
		},
		CollectedAt: systemInfo.CollectedAt,
	}, nil
}

// getSSHConfig loads SSH private key(s) from disk (no SSH agent)
func getSSHConfig(username string) ([]ssh.AuthMethod, error) {
	var authMethods []ssh.AuthMethod

	if username == "" {
		username = "root"
	}

	// IMPORTANT: do NOT use SSH_AUTH_SOCK / ssh-agent here.
	// We rely only on private key files from disk.
	homeCandidates := map[string]struct{}{}

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		homeCandidates[home] = struct{}{}
	}
	if usr, err := user.Current(); err == nil && usr.HomeDir != "" {
		homeCandidates[usr.HomeDir] = struct{}{}
	}
	if homeEnv := os.Getenv("HOME"); homeEnv != "" {
		homeCandidates[homeEnv] = struct{}{}
	}

	// Keep explicit fallbacks for common service user setups.
	homeCandidates["/home/sysadmin007"] = struct{}{}
	homeCandidates["/root"] = struct{}{}

	keyNames := []string{"id_ed25519", "id_rsa", "id_ecdsa"}
	seenPath := map[string]struct{}{}

	for home := range homeCandidates {
		for _, keyName := range keyNames {
			keyPath := filepath.Join(home, ".ssh", keyName)
			if _, seen := seenPath[keyPath]; seen {
				continue
			}
			seenPath[keyPath] = struct{}{}

			key, err := os.ReadFile(keyPath)
			if err != nil {
				continue
			}

			signer, err := ssh.ParsePrivateKey(key)
			if err != nil {
				log.Printf("WARNING: Failed to parse key at %s: %v", keyPath, err)
				continue
			}

			authMethods = append(authMethods, ssh.PublicKeys(signer))
			log.Printf("INFO: Loaded SSH key from %s", keyPath)
		}
	}

	if len(authMethods) == 0 {
		return nil, errors.New("no usable SSH private keys found in ~/.ssh (id_ed25519/id_rsa/id_ecdsa)")
	}

	return authMethods, nil
}

// executeSSHCommand connects to a remote computer via SSH and executes a command
func executeSSHCommand(ip, username, command string) (string, error) {
	if username == "" {
		username = "root"
	}

	authMethods, err := getSSHConfig(username)
	if err != nil {
		log.Printf("ERROR: Failed to get SSH config: %v", err)
		return "", err
	}

	config := &ssh.ClientConfig{
		User:            username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // ⚠️ Only for trusted internal networks
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", net.JoinHostPort(ip, "22"), config)
	if err != nil {
		log.Printf("ERROR: Failed to connect to %s: %v", ip, err)
		return "", fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		log.Printf("ERROR: Failed to create SSH session for %s: %v", ip, err)
		return "", fmt.Errorf("SSH session failed: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(command); err != nil {
		log.Printf("WARNING: SSH command failed on %s: %v", ip, err)
		stderrText := stderr.String()
		if stderrText != "" {
			return stderrText, nil
		}
		return "", fmt.Errorf("command execution failed: %w", err)
	}

	output := stdout.String()
	log.Printf("INFO: SSH command executed on %s - Command: %s - Output length: %d", ip, command, len(output))
	return output, nil
}

// executeTerminal handles POST /api/terminal/execute
func executeTerminal(w http.ResponseWriter, r *http.Request) {
	setCORS(w)

	var req TerminalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid JSON body",
		})
		log.Printf("ERROR: Failed to decode terminal request: %v", err)
		return
	}

	if req.ComputerID == "" || req.Command == "" {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "computerId and command are required",
		})
		return
	}

	computers, err := getComputers()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Failed to retrieve computers",
		})
		return
	}

	var computer *Computer
	for i := range computers {
		if computers[i].ID == req.ComputerID {
			computer = &computers[i]
			break
		}
	}

	if computer == nil {
		writeJSON(w, http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "Computer not found",
		})
		log.Printf("WARNING: Attempt to execute terminal command on non-existent computer: %s", req.ComputerID)
		return
	}

	output, err := executeSSHCommand(computer.IP, computer.Username, req.Command)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("SSH execution failed: %v", err),
		})
		log.Printf("ERROR: SSH command execution failed for %s@%s (%s): %v", computer.Username, computer.Place, computer.IP, err)
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: TerminalResponse{
			Output: output,
		},
	})
	log.Printf("INFO: Terminal command executed on %s@%s (%s) - Command: %s", computer.Username, computer.Place, computer.IP, req.Command)
}

// getComputerSystemInfo handles GET /api/system-info/:id
// Collects host metrics via SSH and persists OS changes to DB.
func getComputerSystemInfo(w http.ResponseWriter, r *http.Request) {
	setCORS(w)

	id := strings.TrimPrefix(r.URL.Path, "/api/system-info/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Missing computer ID",
		})
		return
	}

	computers, err := getComputers()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Failed to retrieve computers",
		})
		return
	}

	var computer *Computer
	for i := range computers {
		if computers[i].ID == id {
			computer = &computers[i]
			break
		}
	}

	if computer == nil {
		writeJSON(w, http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "Computer not found",
		})
		return
	}

	info, err := collectSystemInfo(*computer)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to collect system info via SSH: %v", err),
		})
		log.Printf("ERROR: Failed to collect system info for %s@%s (%s): %v", computer.Username, computer.Place, computer.IP, err)
		return
	}

	// Persist OS version changes; future card refreshes show latest OS.
	if info.OS != "" && info.OS != computer.OS {
		if err := updateComputerOS(computer.ID, info.OS); err != nil {
			log.Printf("WARNING: Failed to persist OS update for ID %s: %v", computer.ID, err)
		}
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    info,
	})
	log.Printf("INFO: System info collected for %s@%s (%s): OS=%s", computer.Username, computer.Place, computer.IP, info.OS)
}

// getCPUOverview handles GET /api/cpu-overview/:id
// Runs basic Linux commands and returns parsed CPU overview + raw command output.
func getCPUOverview(w http.ResponseWriter, r *http.Request) {
	setCORS(w)

	id := strings.TrimPrefix(r.URL.Path, "/api/cpu-overview/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Missing computer ID",
		})
		return
	}

	computers, err := getComputers()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Failed to retrieve computers",
		})
		return
	}

	var computer *Computer
	for i := range computers {
		if computers[i].ID == id {
			computer = &computers[i]
			break
		}
	}

	if computer == nil {
		writeJSON(w, http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "Computer not found",
		})
		return
	}

	overview, err := collectCPUOverview(*computer)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to collect CPU overview via SSH: %v", err),
		})
		log.Printf("ERROR: Failed to collect CPU overview for %s@%s (%s): %v", computer.Username, computer.Place, computer.IP, err)
		return
	}

	if overview.OS != "" && overview.OS != computer.OS {
		if err := updateComputerOS(computer.ID, overview.OS); err != nil {
			log.Printf("WARNING: Failed to persist OS update for ID %s: %v", computer.ID, err)
		}
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    overview,
	})
	log.Printf("INFO: CPU overview collected for %s@%s (%s)", computer.Username, computer.Place, computer.IP)
}

// ============================================================================
// WebSocket SSH Terminal
// ============================================================================

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for internal network use
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// handleTerminalWS handles GET /api/terminal/ws?computerId=xxx
// Upgrades HTTP to WebSocket and bridges it to a persistent SSH PTY session
func handleTerminalWS(w http.ResponseWriter, r *http.Request) {
	computerID := r.URL.Query().Get("computerId")
	if computerID == "" {
		http.Error(w, "computerId query param required", http.StatusBadRequest)
		return
	}

	computers, err := getComputers()
	if err != nil {
		http.Error(w, "Failed to retrieve computers", http.StatusInternalServerError)
		return
	}

	var computer *Computer
	for i := range computers {
		if computers[i].ID == computerID {
			computer = &computers[i]
			break
		}
	}

	if computer == nil {
		http.Error(w, "Computer not found", http.StatusNotFound)
		return
	}

	// Upgrade HTTP → WebSocket
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ERROR: WebSocket upgrade failed for computerId=%s: %v", computerID, err)
		return
	}
	defer wsConn.Close()

	log.Printf("INFO: WebSocket terminal opened for %s@%s (%s)", computer.Username, computer.Place, computer.IP)

	// Let the browser/user know which target we are trying to reach
	wsConn.WriteMessage(
		websocket.TextMessage,
		[]byte(fmt.Sprintf("Connecting to %s@%s via SSH...\r\n", computer.Username, computer.IP)),
	)

	authMethods, err := getSSHConfig(computer.Username)
	if err != nil {
		log.Printf("ERROR: SSH key error for %s@%s (%s): %v", computer.Username, computer.Place, computer.IP, err)
		wsConn.WriteMessage(websocket.TextMessage, []byte("SSH key error: "+err.Error()+"\r\n"))
		return
	}

	sshConfig := &ssh.ClientConfig{
		User:            computer.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // ⚠️ Only for trusted internal networks
		Timeout:         10 * time.Second,
	}

	sshClient, err := ssh.Dial("tcp", net.JoinHostPort(computer.IP, "22"), sshConfig)
	if err != nil {
		log.Printf("ERROR: SSH connection failed for %s@%s (%s): %v", computer.Username, computer.Place, computer.IP, err)
		wsConn.WriteMessage(websocket.TextMessage, []byte("SSH connection failed: "+err.Error()+"\r\n"))
		return
	}
	defer sshClient.Close()

	session, err := sshClient.NewSession()
	if err != nil {
		log.Printf("ERROR: SSH session failed for %s@%s (%s): %v", computer.Username, computer.Place, computer.IP, err)
		wsConn.WriteMessage(websocket.TextMessage, []byte("SSH session failed: "+err.Error()+"\r\n"))
		return
	}
	defer session.Close()

	// Request a PTY — this is what makes it an interactive terminal
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", 40, 120, modes); err != nil {
		log.Printf("ERROR: PTY request failed for %s@%s (%s): %v", computer.Username, computer.Place, computer.IP, err)
		wsConn.WriteMessage(websocket.TextMessage, []byte("PTY request failed: "+err.Error()+"\r\n"))
		return
	}

	sshOut, err := session.StdoutPipe()
	if err != nil {
		log.Printf("ERROR: stdout pipe failed for %s@%s (%s): %v", computer.Username, computer.Place, computer.IP, err)
		wsConn.WriteMessage(websocket.TextMessage, []byte("Pipe error: "+err.Error()+"\r\n"))
		return
	}
	sshErr, err := session.StderrPipe()
	if err != nil {
		log.Printf("ERROR: stderr pipe failed for %s@%s (%s): %v", computer.Username, computer.Place, computer.IP, err)
		wsConn.WriteMessage(websocket.TextMessage, []byte("Pipe error: "+err.Error()+"\r\n"))
		return
	}
	sshIn, err := session.StdinPipe()
	if err != nil {
		log.Printf("ERROR: stdin pipe failed for %s@%s (%s): %v", computer.Username, computer.Place, computer.IP, err)
		wsConn.WriteMessage(websocket.TextMessage, []byte("Pipe error: "+err.Error()+"\r\n"))
		return
	}

	if err := session.Shell(); err != nil {
		log.Printf("ERROR: shell start failed for %s@%s (%s): %v", computer.Username, computer.Place, computer.IP, err)
		wsConn.WriteMessage(websocket.TextMessage, []byte("Shell start failed: "+err.Error()+"\r\n"))
		return
	}

	// Clear message that SSH is ready and where commands go
	wsConn.WriteMessage(
		websocket.TextMessage,
		[]byte(fmt.Sprintf("SSH session established. Commands now run on %s@%s.\r\n\r\n", computer.Username, computer.IP)),
	)

	done := make(chan struct{})

	// SSH stdout → WebSocket
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := sshOut.Read(buf)
			if n > 0 {
				if writeErr := wsConn.WriteMessage(websocket.BinaryMessage, buf[:n]); writeErr != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
		close(done)
	}()

	// SSH stderr → WebSocket
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := sshErr.Read(buf)
			if n > 0 {
				wsConn.WriteMessage(websocket.BinaryMessage, buf[:n])
			}
			if err != nil {
				break
			}
		}
	}()

	// WebSocket → SSH stdin (main loop)
	for {
		_, msg, err := wsConn.ReadMessage()
		if err != nil {
			break
		}
		// Handle terminal resize: {"type":"resize","cols":120,"rows":40}
		if len(msg) > 0 && msg[0] == '{' {
			var resizeMsg struct {
				Type string `json:"type"`
				Cols uint32 `json:"cols"`
				Rows uint32 `json:"rows"`
			}
			if json.Unmarshal(msg, &resizeMsg) == nil && resizeMsg.Type == "resize" {
				session.WindowChange(int(resizeMsg.Rows), int(resizeMsg.Cols))
				continue
			}
		}
		if _, err := sshIn.Write(msg); err != nil {
			break
		}
	}

	<-done
	log.Printf("INFO: WebSocket terminal closed for %s@%s", computer.Username, computer.Place)
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

	switch {
	case path == "/api/computers" && r.Method == http.MethodGet:
		listComputers(w, r)
	case path == "/api/computers" && r.Method == http.MethodPost:
		addComputer(w, r)
	case strings.HasPrefix(path, "/api/computers/") && r.Method == http.MethodPut:
		editComputer(w, r)
	case strings.HasPrefix(path, "/api/computers/") && r.Method == http.MethodDelete:
		deleteComputerHandler(w, r)
	case strings.HasPrefix(path, "/api/ping/") && r.Method == http.MethodGet:
		pingOne(w, r)
	case path == "/api/ping-all" && r.Method == http.MethodGet:
		pingAll(w, r)
	case strings.HasPrefix(path, "/api/system-info/") && r.Method == http.MethodGet:
		getComputerSystemInfo(w, r)
	case strings.HasPrefix(path, "/api/cpu-overview/") && r.Method == http.MethodGet:
		getCPUOverview(w, r)
	case path == "/api/terminal/execute" && r.Method == http.MethodPost:
		executeTerminal(w, r)
	case path == "/api/terminal/ws" && r.Method == http.MethodGet:
		handleTerminalWS(w, r) // ✅ WebSocket route correctly placed here
	default:
		http.FileServer(http.Dir("../frontend")).ServeHTTP(w, r)
	}
}

// ============================================================================
// Main
// ============================================================================

func main() {
	// Initialize database
	if err := initDatabase(dbPath); err != nil {
		log.Fatalf("FATAL: Failed to initialize database: %v", err)
	}
	defer closeDatabase()

	// Register router
	http.HandleFunc("/", router)

	port := ":8081"
	log.Printf("========================================")
	log.Printf("Network PC Monitoring System - Started")
	log.Printf("Server running at http://localhost%s", port)
	log.Printf("Database: %s", dbPath)
	log.Printf("========================================")

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("FATAL: Server failed to start: %v", err)
	}
}
