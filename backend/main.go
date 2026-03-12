package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/creack/pty"
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
	setCORSHeaders(w)
	w.Header().Set("Content-Type", "application/json")
}

// setCORSHeaders configures only CORS headers (without forcing response type).
func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
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

type FileTransferRoot struct {
	Label  string `json:"label"`
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
}

type FileTransferEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	IsDir      bool   `json:"isDir"`
	SizeBytes  int64  `json:"sizeBytes"`
	ModifiedAt string `json:"modifiedAt"`
}

type FileTransferListResponse struct {
	ComputerID  string              `json:"computerId"`
	CurrentPath string              `json:"currentPath"`
	HomePath    string              `json:"homePath"`
	ParentPath  string              `json:"parentPath"`
	Roots       []FileTransferRoot  `json:"roots"`
	Entries     []FileTransferEntry `json:"entries"`
}

type FileTransferCopyRequest struct {
	SourceComputerID string `json:"sourceComputerId"`
	SourcePath       string `json:"sourcePath"`
	TargetComputerID string `json:"targetComputerId"`
	TargetPath       string `json:"targetPath,omitempty"`
	Mode             string `json:"mode,omitempty"` // "copy" (default), "merge", "merge_newer"
}

type FileTransferUndoRequest struct {
	ComputerID string `json:"computerId"`
}

// normalizeComputerHost strips spaces/brackets and optional :port so host/IP
// comparisons are consistent.
func normalizeComputerHost(raw string) string {
	host := strings.TrimSpace(raw)
	if host == "" {
		return ""
	}

	// Handle bracketed IPv6 with optional port, e.g. [::1]:22.
	if strings.HasPrefix(host, "[") {
		if idx := strings.Index(host, "]"); idx > 0 {
			inner := host[1:idx]
			rest := host[idx+1:]
			if rest == "" || strings.HasPrefix(rest, ":") {
				return strings.TrimSpace(inner)
			}
		}
	}

	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}

	return strings.Trim(strings.TrimSpace(host), "[]")
}

func localInterfaceIPs() map[string]struct{} {
	localIPs := make(map[string]struct{})

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Printf("WARNING: Failed to read local interface IPs: %v", err)
		return localIPs
	}

	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		default:
			continue
		}

		if ip == nil {
			continue
		}

		localIPs[ip.String()] = struct{}{}
		if v4 := ip.To4(); v4 != nil {
			localIPs[v4.String()] = struct{}{}
		}
	}

	return localIPs
}

// isServerComputer returns true when the computer entry points to this server
// itself (localhost, loopback, hostname, or any local interface IP).
func isServerComputer(c Computer) bool {
	targetHost := normalizeComputerHost(c.IP)
	if targetHost == "" {
		return false
	}

	if strings.EqualFold(targetHost, "localhost") {
		return true
	}

	if hostname, err := os.Hostname(); err == nil {
		if strings.EqualFold(targetHost, normalizeComputerHost(hostname)) {
			return true
		}
	}

	targetIP := net.ParseIP(targetHost)
	if targetIP == nil {
		return false
	}
	if targetIP.IsLoopback() {
		return true
	}

	localIPs := localInterfaceIPs()
	if _, ok := localIPs[targetIP.String()]; ok {
		return true
	}
	if targetV4 := targetIP.To4(); targetV4 != nil {
		if _, ok := localIPs[targetV4.String()]; ok {
			return true
		}
	}

	return false
}

func getComputerByID(id string) (*Computer, error) {
	computers, err := getComputers()
	if err != nil {
		return nil, err
	}

	for i := range computers {
		if computers[i].ID == id {
			return &computers[i], nil
		}
	}
	return nil, nil
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func sanitizeDownloadName(name string) string {
	clean := strings.TrimSpace(name)
	clean = strings.ReplaceAll(clean, "/", "_")
	clean = strings.ReplaceAll(clean, "\\", "_")
	if clean == "" || clean == "." || clean == ".." {
		return "download"
	}
	return clean
}

func pathWithinHome(targetPath, homePath string) bool {
	targetPath = pathpkg.Clean(targetPath)
	homePath = pathpkg.Clean(homePath)
	return targetPath == homePath || strings.HasPrefix(targetPath, homePath+"/")
}

func normalizeComputerPath(rawPath, homePath string) (string, error) {
	homePath = pathpkg.Clean(strings.TrimSpace(homePath))
	if homePath == "" || !strings.HasPrefix(homePath, "/") {
		return "", errors.New("invalid home path")
	}

	p := strings.TrimSpace(rawPath)
	if p == "" {
		return "", errors.New("path is required")
	}

	var absolute string
	switch {
	case p == "~":
		absolute = homePath
	case strings.HasPrefix(p, "~/"):
		absolute = pathpkg.Join(homePath, strings.TrimPrefix(p, "~/"))
	case strings.HasPrefix(p, "/"):
		absolute = pathpkg.Clean(p)
	default:
		absolute = pathpkg.Join(homePath, p)
	}

	if !strings.HasPrefix(absolute, "/") {
		absolute = "/" + strings.TrimPrefix(absolute, "/")
	}

	if !pathWithinHome(absolute, homePath) {
		return "", errors.New("path must stay inside the user home directory")
	}

	return absolute, nil
}

func getComputerHomePath(computer Computer) (string, error) {
	out, err := executeCommandForComputer(computer, `printf %s "$HOME"`)
	if err != nil {
		return "", err
	}

	home := strings.TrimSpace(out)
	if home == "" || !strings.HasPrefix(home, "/") {
		return "", errors.New("failed to resolve home directory")
	}
	return pathpkg.Clean(home), nil
}

func remotePathType(computer Computer, absolutePath string) (string, error) {
	q := shellQuote(absolutePath)
	cmd := fmt.Sprintf(`if [ -d %s ]; then echo dir; elif [ -f %s ]; then echo file; else echo missing; fi`, q, q)
	out, err := executeCommandForComputer(computer, cmd)
	if err != nil {
		return "", err
	}
	kind := strings.TrimSpace(out)
	switch kind {
	case "dir", "file", "missing":
		return kind, nil
	default:
		return "", fmt.Errorf("unexpected path type output: %q", kind)
	}
}

func buildFileTransferRoots(computer Computer, homePath string) []FileTransferRoot {
	candidates := []FileTransferRoot{
		{Label: "Documents", Path: pathpkg.Join(homePath, "Documents")},
		{Label: "Downloads", Path: pathpkg.Join(homePath, "Downloads")},
		{Label: "Home", Path: homePath, Exists: true},
	}

	for i := range candidates {
		if candidates[i].Label == "Home" {
			continue
		}
		kind, err := remotePathType(computer, candidates[i].Path)
		candidates[i].Exists = err == nil && kind == "dir"
	}

	return candidates
}

func chooseDefaultBrowsePath(roots []FileTransferRoot, homePath string) string {
	for _, root := range roots {
		if root.Exists && root.Path != "" && root.Label != "Home" {
			return root.Path
		}
	}
	return homePath
}

func chooseDefaultPastePath(computer Computer, homePath string) string {
	downloads := pathpkg.Join(homePath, "Downloads")
	kind, err := remotePathType(computer, downloads)
	if err == nil && kind == "dir" {
		return downloads
	}
	return homePath
}

func listDirectoryEntries(computer Computer, dirPath string, homePath string) ([]FileTransferEntry, error) {
	q := shellQuote(dirPath)
	cmd := fmt.Sprintf(
		`if [ ! -d %s ]; then echo "__NOT_DIRECTORY__"; exit 0; fi; find %s -mindepth 1 -maxdepth 1 -printf '%%y\t%%f\t%%p\t%%s\t%%TY-%%Tm-%%Td %%TH:%%TM\n' 2>/dev/null | sort -f`,
		q, q,
	)

	out, err := executeCommandForComputer(computer, cmd)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(out) == "__NOT_DIRECTORY__" {
		return nil, errors.New("selected path is not a directory")
	}

	entries := make([]FileTransferEntry, 0)
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "\t", 5)
		if len(parts) < 5 {
			continue
		}

		entryPath := strings.TrimSpace(parts[2])
		if entryPath == "" || !pathWithinHome(entryPath, homePath) {
			continue
		}

		sizeBytes, _ := strconv.ParseInt(strings.TrimSpace(parts[3]), 10, 64)
		entries = append(entries, FileTransferEntry{
			Name:       strings.TrimSpace(parts[1]),
			Path:       entryPath,
			IsDir:      strings.TrimSpace(parts[0]) == "d",
			SizeBytes:  sizeBytes,
			ModifiedAt: strings.TrimSpace(parts[4]),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir && !entries[j].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	return entries, nil
}

func executeCommandForComputer(computer Computer, command string) (string, error) {
	if isServerComputer(computer) {
		return executeLocalCommand(command)
	}
	return executeSSHCommand(computer.IP, computer.Username, command)
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

type CPUOverview struct {
	ComputerID         string  `json:"computerId"`
	Place              string  `json:"place"`
	Username           string  `json:"username"`
	IP                 string  `json:"ip"`
	OS                 string  `json:"os"`
	Kernel             string  `json:"kernel"`
	Uptime             string  `json:"uptime"`
	Architecture       string  `json:"architecture"`
	CPUModel           string  `json:"cpuModel"`
	CoreCount          int     `json:"coreCount"`
	ThreadsPerCore     int     `json:"threadsPerCore"`
	SocketCount        int     `json:"socketCount"`
	Load1              float64 `json:"load1"`
	Load5              float64 `json:"load5"`
	Load15             float64 `json:"load15"`
	CPUUsagePercent    float64 `json:"cpuUsagePercent"`
	MemoryTotalGB      float64 `json:"memoryTotalGB"`
	MemoryUsedGB       float64 `json:"memoryUsedGB"`
	MemoryUsagePercent float64 `json:"memoryUsagePercent"`
	DiskTotalGB        float64 `json:"diskTotalGB"`
	DiskUsedGB         float64 `json:"diskUsedGB"`
	DiskUsagePercent   float64 `json:"diskUsagePercent"`
	CollectedAt        string  `json:"collectedAt"`
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
	output, err := executeCommandForComputer(computer, command)
	if err != nil {
		return SystemInfo{}, err
	}

	values := parseKeyValueOutput(output)
	if len(values) == 0 {
		return SystemInfo{}, errors.New("unable to parse system metrics from command output")
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
	summaryOut, err := executeCommandForComputer(computer, summaryCommand)
	if err != nil {
		return CPUOverview{}, err
	}

	summary := parseKeyValueOutput(summaryOut)
	if len(summary) == 0 {
		return CPUOverview{}, errors.New("unable to parse cpu overview metrics from command output")
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
		CollectedAt:        systemInfo.CollectedAt,
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

func streamSSHCommandOutput(ip, username, command string, writer io.Writer) (string, error) {
	if username == "" {
		username = "root"
	}

	authMethods, err := getSSHConfig(username)
	if err != nil {
		return "", err
	}

	config := &ssh.ClientConfig{
		User:            username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // ⚠️ Only for trusted internal networks
		Timeout:         20 * time.Second,
	}

	client, err := ssh.Dial("tcp", net.JoinHostPort(ip, "22"), config)
	if err != nil {
		return "", fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("SSH session failed: %w", err)
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return "", err
	}

	var stderr bytes.Buffer
	session.Stderr = &stderr

	if err := session.Start(command); err != nil {
		return stderr.String(), err
	}

	if _, err := io.Copy(writer, stdout); err != nil {
		return stderr.String(), err
	}

	if err := session.Wait(); err != nil {
		return stderr.String(), err
	}

	return stderr.String(), nil
}

func streamLocalCommandOutput(command string, writer io.Writer) (string, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	cmd := exec.Command(shell, "-lc", command)
	cmd.Env = os.Environ()
	cmd.Stdout = writer

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return stderr.String(), err
	}

	return stderr.String(), nil
}

func streamCommandOutputForComputer(computer Computer, command string, writer io.Writer) (string, error) {
	if isServerComputer(computer) {
		return streamLocalCommandOutput(command, writer)
	}
	return streamSSHCommandOutput(computer.IP, computer.Username, command, writer)
}

func streamSSHCommandInput(ip, username, command string, reader io.Reader) (string, error) {
	if username == "" {
		username = "root"
	}

	authMethods, err := getSSHConfig(username)
	if err != nil {
		return "", err
	}

	config := &ssh.ClientConfig{
		User:            username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // ⚠️ Only for trusted internal networks
		Timeout:         20 * time.Second,
	}

	client, err := ssh.Dial("tcp", net.JoinHostPort(ip, "22"), config)
	if err != nil {
		return "", fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("SSH session failed: %w", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return "", err
	}

	session.Stdout = io.Discard
	var stderr bytes.Buffer
	session.Stderr = &stderr

	if err := session.Start(command); err != nil {
		return stderr.String(), err
	}

	if _, err := io.Copy(stdin, reader); err != nil {
		_ = stdin.Close()
		return stderr.String(), err
	}
	_ = stdin.Close()

	if err := session.Wait(); err != nil {
		return stderr.String(), err
	}

	return stderr.String(), nil
}

func streamLocalCommandInput(command string, reader io.Reader) (string, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	cmd := exec.Command(shell, "-lc", command)
	cmd.Env = os.Environ()
	cmd.Stdin = reader
	cmd.Stdout = io.Discard

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return stderr.String(), err
	}

	return stderr.String(), nil
}

func streamCommandInputForComputer(computer Computer, command string, reader io.Reader) (string, error) {
	if isServerComputer(computer) {
		return streamLocalCommandInput(command, reader)
	}
	return streamSSHCommandInput(computer.IP, computer.Username, command, reader)
}

func executeCommandStrictForComputer(computer Computer, command string) (string, error) {
	var stdout bytes.Buffer
	stderr, err := streamCommandOutputForComputer(computer, command, &stdout)
	if err != nil {
		if strings.TrimSpace(stderr) != "" {
			return strings.TrimSpace(stderr), err
		}
		if stdout.Len() > 0 {
			return strings.TrimSpace(stdout.String()), err
		}
		return "", err
	}

	return strings.TrimSpace(stdout.String()), nil
}

// executeLocalCommand runs a shell command on the local server instead of over SSH.
// Used when the selected computer is the monitoring server itself.
func executeLocalCommand(command string) (string, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	cmd := exec.Command(shell, "-lc", command)
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Printf("WARNING: Local command failed: %v", err)
		if stderr.Len() > 0 {
			return stderr.String(), nil
		}
		return "", fmt.Errorf("local command failed: %w", err)
	}

	return stdout.String(), nil
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
	output, err := executeCommandForComputer(*computer, req.Command)
	if err != nil {
		executionType := "SSH"
		if isServerComputer(*computer) {
			executionType = "local shell"
		}
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("%s execution failed: %v", executionType, err),
		})
		log.Printf("ERROR: %s command execution failed for %s@%s (%s): %v", executionType, computer.Username, computer.Place, computer.IP, err)
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

// listComputerFiles handles GET /api/file-transfer/list?computerId=:id&path=:path
func listComputerFiles(w http.ResponseWriter, r *http.Request) {
	setCORS(w)

	computerID := strings.TrimSpace(r.URL.Query().Get("computerId"))
	if computerID == "" {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "computerId is required",
		})
		return
	}

	computer, err := getComputerByID(computerID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Failed to retrieve computers",
		})
		return
	}
	if computer == nil {
		writeJSON(w, http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "Computer not found",
		})
		return
	}

	homePath, err := getComputerHomePath(*computer)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to resolve home directory: %v", err),
		})
		return
	}

	roots := buildFileTransferRoots(*computer, homePath)
	pathArg := strings.TrimSpace(r.URL.Query().Get("path"))

	currentPath := chooseDefaultBrowsePath(roots, homePath)
	if pathArg != "" {
		currentPath, err = normalizeComputerPath(pathArg, homePath)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}
	}

	pathType, err := remotePathType(*computer, currentPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to inspect selected path: %v", err),
		})
		return
	}
	if pathType == "missing" {
		writeJSON(w, http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "Selected path does not exist",
		})
		return
	}
	if pathType != "dir" {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Selected path must be a directory",
		})
		return
	}

	entries, err := listDirectoryEntries(*computer, currentPath, homePath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to list directory: %v", err),
		})
		return
	}

	parentPath := ""
	if currentPath != homePath {
		candidate := pathpkg.Dir(currentPath)
		if pathWithinHome(candidate, homePath) {
			parentPath = candidate
		}
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: FileTransferListResponse{
			ComputerID:  computer.ID,
			CurrentPath: currentPath,
			HomePath:    homePath,
			ParentPath:  parentPath,
			Roots:       roots,
			Entries:     entries,
		},
	})
}

// downloadComputerPath handles GET /api/file-transfer/download?computerId=:id&path=:path
func downloadComputerPath(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	sendError := func(code int, message string) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, code, APIResponse{
			Success: false,
			Error:   message,
		})
	}

	computerID := strings.TrimSpace(r.URL.Query().Get("computerId"))
	pathArg := strings.TrimSpace(r.URL.Query().Get("path"))
	if computerID == "" || pathArg == "" {
		sendError(http.StatusBadRequest, "computerId and path are required")
		return
	}

	computer, err := getComputerByID(computerID)
	if err != nil {
		sendError(http.StatusInternalServerError, "Failed to retrieve computers")
		return
	}
	if computer == nil {
		sendError(http.StatusNotFound, "Computer not found")
		return
	}

	homePath, err := getComputerHomePath(*computer)
	if err != nil {
		sendError(http.StatusInternalServerError, fmt.Sprintf("Failed to resolve home directory: %v", err))
		return
	}

	absolutePath, err := normalizeComputerPath(pathArg, homePath)
	if err != nil {
		sendError(http.StatusBadRequest, err.Error())
		return
	}

	pathType, err := remotePathType(*computer, absolutePath)
	if err != nil {
		sendError(http.StatusInternalServerError, fmt.Sprintf("Failed to inspect selected path: %v", err))
		return
	}
	if pathType == "missing" {
		sendError(http.StatusNotFound, "Selected path does not exist")
		return
	}
	if pathType != "dir" && pathType != "file" {
		sendError(http.StatusBadRequest, "Only files or directories can be downloaded")
		return
	}

	baseName := sanitizeDownloadName(pathpkg.Base(absolutePath))
	contentType := "application/octet-stream"
	command := fmt.Sprintf("cat %s", shellQuote(absolutePath))
	downloadName := baseName

	if pathType == "dir" {
		command = fmt.Sprintf(
			"tar -czf - -C %s %s",
			shellQuote(pathpkg.Dir(absolutePath)),
			shellQuote(pathpkg.Base(absolutePath)),
		)
		contentType = "application/gzip"
		downloadName = baseName + ".tar.gz"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", downloadName))
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if stderr, err := streamCommandOutputForComputer(*computer, command, w); err != nil {
		log.Printf(
			"ERROR: Download failed for %s@%s (%s) path=%s err=%v stderr=%s",
			computer.Username,
			computer.Place,
			computer.IP,
			absolutePath,
			err,
			strings.TrimSpace(stderr),
		)
	}
}

// copyComputerPath handles POST /api/file-transfer/copy
// Copies one file/folder from source computer to target computer.
func copyComputerPath(w http.ResponseWriter, r *http.Request) {
	setCORS(w)

	var req FileTransferCopyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid JSON body",
		})
		return
	}

	req.SourceComputerID = strings.TrimSpace(req.SourceComputerID)
	req.TargetComputerID = strings.TrimSpace(req.TargetComputerID)
	req.SourcePath = strings.TrimSpace(req.SourcePath)
	req.TargetPath = strings.TrimSpace(req.TargetPath)
	req.Mode = strings.ToLower(strings.TrimSpace(req.Mode))
	if req.Mode == "" {
		req.Mode = "copy"
	}

	if req.SourceComputerID == "" || req.TargetComputerID == "" || req.SourcePath == "" {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "sourceComputerId, sourcePath and targetComputerId are required",
		})
		return
	}

	if req.Mode != "copy" && req.Mode != "merge" && req.Mode != "merge_newer" {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "mode must be either \"copy\", \"merge\" or \"merge_newer\"",
		})
		return
	}

	sourceComputer, err := getComputerByID(req.SourceComputerID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Failed to retrieve source computer",
		})
		return
	}
	if sourceComputer == nil {
		writeJSON(w, http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "Source computer not found",
		})
		return
	}

	targetComputer, err := getComputerByID(req.TargetComputerID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Failed to retrieve target computer",
		})
		return
	}
	if targetComputer == nil {
		writeJSON(w, http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "Target computer not found",
		})
		return
	}

	sourceHome, err := getComputerHomePath(*sourceComputer)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to resolve source home: %v", err),
		})
		return
	}

	targetHome, err := getComputerHomePath(*targetComputer)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to resolve target home: %v", err),
		})
		return
	}

	sourcePath, err := normalizeComputerPath(req.SourcePath, sourceHome)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	targetRawPath := req.TargetPath
	if targetRawPath == "" {
		targetRawPath = chooseDefaultPastePath(*targetComputer, targetHome)
	}

	targetPath, err := normalizeComputerPath(targetRawPath, targetHome)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	sourceType, err := remotePathType(*sourceComputer, sourcePath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to inspect source path: %v", err),
		})
		return
	}
	if sourceType == "missing" {
		writeJSON(w, http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "Source path does not exist",
		})
		return
	}
	if sourceType != "file" && sourceType != "dir" {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Source path must be a file or directory",
		})
		return
	}

	targetType, err := remotePathType(*targetComputer, targetPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to inspect target path: %v", err),
		})
		return
	}
	if targetType != "dir" {
		createTargetCmd := fmt.Sprintf("mkdir -p %s", shellQuote(targetPath))
		stderr, mkdirErr := executeCommandStrictForComputer(*targetComputer, createTargetCmd)
		if mkdirErr != nil {
			writeJSON(w, http.StatusInternalServerError, APIResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to create target directory: %v %s", mkdirErr, strings.TrimSpace(stderr)),
			})
			return
		}
	}

	if req.SourceComputerID == req.TargetComputerID {
		var copyCmd string
		if req.Mode == "merge_newer" {
			// Merge-newer on the same computer with undo support via rsync backups.
			// Works for both files and directories.
			rsyncScript := fmt.Sprintf(`bash -lc %s`, shellQuote(fmt.Sprintf(`
set -euo pipefail

SRC=%s
DEST=%s

command -v rsync >/dev/null 2>&1 || { echo "rsync is required for merge_newer + undo but is not installed"; exit 2; }
mkdir -p "$DEST"

UNDO_BASE="$HOME/.pc-monitoring-undo"
mkdir -p "$UNDO_BASE/ops"

OP_ID="$(date +%%Y%%m%%d_%%H%%M%%S)_$RANDOM"
OP_DIR="$UNDO_BASE/ops/$OP_ID"
BACKUP_DIR="$OP_DIR/backup"
mkdir -p "$BACKUP_DIR"
printf '%%s' "$DEST" > "$OP_DIR/target.txt"

RSYNC_LOG="$OP_DIR/rsync.log"
if [ -d "$SRC" ]; then
  rsync -a --update --backup --backup-dir="$BACKUP_DIR" --out-format='%%i|%%n' "$SRC"/ "$DEST"/ | tee "$RSYNC_LOG" >/dev/null
else
  rsync -a --update --backup --backup-dir="$BACKUP_DIR" --out-format='%%i|%%n' "$SRC" "$DEST"/ | tee "$RSYNC_LOG" >/dev/null
fi

CREATED_LIST="$OP_DIR/created.txt"
awk -F'\\|' '$1 ~ /^>\\+\\+\\+\\+\\+\\+\\+\\+\\+/ {print $2}' "$RSYNC_LOG" > "$CREATED_LIST" || true

echo "$OP_ID" > "$UNDO_BASE/last_op"
echo "OK:$OP_ID"
`, shellQuote(sourcePath), shellQuote(targetPath)))))

			copyCmd = rsyncScript
		} else if req.Mode == "merge" && sourceType == "dir" {
			// Merge folder contents into targetPath (no extra subfolder).
			copyCmd = fmt.Sprintf(
				"mkdir -p %s && cp -a %s/. %s/",
				shellQuote(targetPath),
				shellQuote(sourcePath),
				shellQuote(targetPath),
			)
		} else {
			// Default behaviour: copy file or folder into targetPath (creates subfolder for directories).
			copyCmd = fmt.Sprintf(
				"mkdir -p %s && cp -a %s %s",
				shellQuote(targetPath),
				shellQuote(sourcePath),
				shellQuote(targetPath),
			)
		}
		if stderr, copyErr := executeCommandStrictForComputer(*sourceComputer, copyCmd); copyErr != nil {
			writeJSON(w, http.StatusInternalServerError, APIResponse{
				Success: false,
				Error:   fmt.Sprintf("Copy failed: %v %s", copyErr, strings.TrimSpace(stderr)),
			})
			return
		}

		writeJSON(w, http.StatusOK, APIResponse{
			Success: true,
			Data: map[string]string{
				"sourceComputerId": req.SourceComputerID,
				"targetComputerId": req.TargetComputerID,
				"sourcePath":       sourcePath,
				"targetPath":       targetPath,
			},
		})
		return
	}

	tmpFile, err := os.CreateTemp("", "pc-monitor-transfer-*.tar")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to create temp archive: %v", err),
		})
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	var createArchiveCmd string
	if (req.Mode == "merge" || req.Mode == "merge_newer") && sourceType == "dir" {
		// For merge mode, archive only the contents of the folder so they can be
		// extracted directly into targetPath without creating an extra subfolder.
		createArchiveCmd = fmt.Sprintf(
			"tar -cf - -C %s .",
			shellQuote(sourcePath),
		)
	} else {
		// Default behaviour: archive the file or folder name so it appears as a
		// sub-item under the target directory.
		createArchiveCmd = fmt.Sprintf(
			"tar -cf - -C %s %s",
			shellQuote(pathpkg.Dir(sourcePath)),
			shellQuote(pathpkg.Base(sourcePath)),
		)
	}
	if stderr, archiveErr := streamCommandOutputForComputer(*sourceComputer, createArchiveCmd, tmpFile); archiveErr != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to read source data: %v %s", archiveErr, strings.TrimSpace(stderr)),
		})
		return
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to rewind transfer archive: %v", err),
		})
		return
	}

	if req.Mode == "merge_newer" && sourceType == "dir" {
		// Merge-newer across computers with undo support:
		// 1) extract tar to a temp dir
		// 2) rsync --update --backup into targetPath
		// 3) record created files list so we can undo later
		opScript := fmt.Sprintf(`bash -lc %s`, shellQuote(fmt.Sprintf(`
set -euo pipefail

TARGET=%s

command -v rsync >/dev/null 2>&1 || { echo "rsync is required for merge_newer + undo but is not installed"; exit 2; }

UNDO_BASE="$HOME/.pc-monitoring-undo"
mkdir -p "$UNDO_BASE/ops"

OP_ID="$(date +%%Y%%m%%d_%%H%%M%%S)_$RANDOM"
OP_DIR="$UNDO_BASE/ops/$OP_ID"
BACKUP_DIR="$OP_DIR/backup"
mkdir -p "$BACKUP_DIR"
printf '%%s' "$TARGET" > "$OP_DIR/target.txt"

TMP_DIR="$(mktemp -d)"
cleanup() { rm -rf "$TMP_DIR"; }
trap cleanup EXIT

tar -xf - -C "$TMP_DIR"

# rsync output:
# %i = itemized changes, %n = relative path
RSYNC_LOG="$OP_DIR/rsync.log"
rsync -a --update --backup --backup-dir="$BACKUP_DIR" --out-format='%%i|%%n' "$TMP_DIR"/ "$TARGET"/ | tee "$RSYNC_LOG" >/dev/null

# Record created files so we can delete them on undo.
# rsync itemize format: if first char is '>' and the second is 'f' or 'd', it's a transfer to receiver.
# For newly created files/directories the "new" flag shows as '+++++++++' in %i.
CREATED_LIST="$OP_DIR/created.txt"
awk -F'\\|' '$1 ~ /^>\\+\\+\\+\\+\\+\\+\\+\\+\\+/ {print $2}' "$RSYNC_LOG" > "$CREATED_LIST" || true

echo "$OP_ID" > "$UNDO_BASE/last_op"
echo "OK:$OP_ID"
`, shellQuote(targetPath)))))

		if stderr, extractErr := streamCommandInputForComputer(*targetComputer, opScript, tmpFile); extractErr != nil {
			writeJSON(w, http.StatusInternalServerError, APIResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to merge_newer into target: %v %s", extractErr, strings.TrimSpace(stderr)),
			})
			return
		}
	} else {
		extractArchiveCmd := fmt.Sprintf(
			"mkdir -p %s && tar -xf - -C %s",
			shellQuote(targetPath),
			shellQuote(targetPath),
		)
		if stderr, extractErr := streamCommandInputForComputer(*targetComputer, extractArchiveCmd, tmpFile); extractErr != nil {
			writeJSON(w, http.StatusInternalServerError, APIResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to write target data: %v %s", extractErr, strings.TrimSpace(stderr)),
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]string{
			"sourceComputerId": req.SourceComputerID,
			"targetComputerId": req.TargetComputerID,
			"sourcePath":       sourcePath,
			"targetPath":       targetPath,
			"mode":             req.Mode,
		},
	})
}

// undoLastMerge handles POST /api/file-transfer/undo
// Restores the last merge_newer operation on the selected computer.
func undoLastMerge(w http.ResponseWriter, r *http.Request) {
	setCORS(w)

	var req FileTransferUndoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, APIResponse{Success: false, Error: "Invalid JSON body"})
		return
	}
	req.ComputerID = strings.TrimSpace(req.ComputerID)
	if req.ComputerID == "" {
		writeJSON(w, http.StatusBadRequest, APIResponse{Success: false, Error: "computerId is required"})
		return
	}

	computer, err := getComputerByID(req.ComputerID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{Success: false, Error: "Failed to retrieve computers"})
		return
	}
	if computer == nil {
		writeJSON(w, http.StatusNotFound, APIResponse{Success: false, Error: "Computer not found"})
		return
	}

	undoCmd := `bash -lc '
set -euo pipefail
command -v rsync >/dev/null 2>&1 || { echo "rsync is required for undo but is not installed"; exit 2; }

UNDO_BASE="$HOME/.pc-monitoring-undo"
LAST_FILE="$UNDO_BASE/last_op"
if [ ! -f "$LAST_FILE" ]; then
  echo "No merge operation to undo."
  exit 3
fi

OP_ID="$(cat "$LAST_FILE" | tr -d "\r\n")"
if [ -z "$OP_ID" ]; then
  echo "No merge operation to undo."
  exit 3
fi

OP_DIR="$UNDO_BASE/ops/$OP_ID"
BACKUP_DIR="$OP_DIR/backup"
CREATED_LIST="$OP_DIR/created.txt"
RSYNC_LOG="$OP_DIR/rsync.log"

if [ ! -d "$OP_DIR" ] || [ ! -d "$BACKUP_DIR" ]; then
  echo "Undo data missing for last operation."
  exit 4
fi

# Determine target directory used during merge by reading rsync log context.
# We can’t reliably infer it later, so we store it if present.
TARGET_FILE="$OP_DIR/target.txt"
if [ ! -f "$TARGET_FILE" ]; then
  echo "Undo target path missing."
  exit 4
fi
TARGET="$(cat "$TARGET_FILE" | tr -d "\r\n")"
if [ -z "$TARGET" ]; then
  echo "Undo target path missing."
  exit 4
fi

# 1) Restore overwritten/updated files from backup dir.
rsync -a "$BACKUP_DIR"/ "$TARGET"/

# 2) Remove files/dirs created by the merge.
if [ -f "$CREATED_LIST" ]; then
  # Delete files first, then directories (deepest-first).
  while IFS= read -r rel; do
    [ -z "$rel" ] && continue
    rm -f "$TARGET/$rel" 2>/dev/null || true
  done < "$CREATED_LIST"

  # Now try removing any empty directories that were created.
  tac "$CREATED_LIST" | while IFS= read -r rel; do
    [ -z "$rel" ] && continue
    rmdir "$TARGET/$rel" 2>/dev/null || true
  done || true
fi

rm -f "$LAST_FILE"
echo "OK"
'`

	// Run on the selected computer.
	out, cmdErr := executeCommandStrictForComputer(*computer, undoCmd)
	if cmdErr != nil {
		writeJSON(w, http.StatusInternalServerError, APIResponse{Success: false, Error: strings.TrimSpace(out)})
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: map[string]string{"computerId": req.ComputerID, "result": strings.TrimSpace(out)}})
}

// getComputerSystemInfo handles GET /api/system-info/:id
// Collects host metrics via local shell (for this server) or SSH.
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
			Error:   fmt.Sprintf("Failed to collect system info: %v", err),
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
// Runs basic Linux commands and returns parsed CPU/RAM/storage/OS metrics.
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
			Error:   fmt.Sprintf("Failed to collect CPU overview: %v", err),
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

// handleLocalTerminal bridges a WebSocket connection to a local shell PTY
// running on the monitoring server itself (no SSH involved).
func handleLocalTerminal(wsConn *websocket.Conn, computer *Computer) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	cmd := exec.Command(shell)
	cmd.Env = os.Environ()
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		cmd.Dir = home
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		log.Printf("ERROR: Failed to start local PTY shell: %v", err)
		wsConn.WriteMessage(websocket.TextMessage, []byte("Failed to start local shell: "+err.Error()+"\r\n"))
		return
	}
	defer func() {
		_ = ptmx.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_, _ = cmd.Process.Wait()
	}()

	wsConn.WriteMessage(
		websocket.TextMessage,
		[]byte(fmt.Sprintf("Local shell established. Commands now run on %s@%s (server).\r\n\r\n", computer.Username, computer.IP)),
	)

	done := make(chan struct{})

	// PTY → WebSocket
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
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

	// WebSocket → PTY
	for {
		_, msg, err := wsConn.ReadMessage()
		if err != nil {
			break
		}

		// Handle terminal resize: {"type":"resize","cols":120,"rows":40}
		if len(msg) > 0 && msg[0] == '{' {
			var resizeMsg struct {
				Type string `json:"type"`
				Cols uint16 `json:"cols"`
				Rows uint16 `json:"rows"`
			}
			if json.Unmarshal(msg, &resizeMsg) == nil && resizeMsg.Type == "resize" {
				_ = pty.Setsize(ptmx, &pty.Winsize{Cols: resizeMsg.Cols, Rows: resizeMsg.Rows})
				continue
			}
		}

		if _, err := ptmx.Write(msg); err != nil {
			break
		}
	}

	<-done
	log.Printf("INFO: Local WebSocket terminal closed for server user %s", computer.Username)
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

	// If this entry represents the monitoring server itself, do NOT use SSH.
	// Instead, attach the WebSocket directly to a local shell PTY so the server
	// uses its own terminal while keeping the exact same frontend UI.
	if isServerComputer(*computer) {
		wsConn.WriteMessage(
			websocket.TextMessage,
			[]byte(fmt.Sprintf("Connecting to %s@%s (local server shell)...\r\n", computer.Username, computer.IP)),
		)
		handleLocalTerminal(wsConn, computer)
		return
	}

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
	case path == "/api/file-transfer/list" && r.Method == http.MethodGet:
		listComputerFiles(w, r)
	case path == "/api/file-transfer/download" && r.Method == http.MethodGet:
		downloadComputerPath(w, r)
	case path == "/api/file-transfer/copy" && r.Method == http.MethodPost:
		copyComputerPath(w, r)
	case path == "/api/file-transfer/undo" && r.Method == http.MethodPost:
		undoLastMerge(w, r)
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
