# System Architecture

## Overview

Network PC Monitoring System is a two-tier web application consisting of:

1. **Backend Server**: Go HTTP server providing REST APIs
2. **Frontend Client**: Vanilla JavaScript SPA with responsive UI

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                        Client Browser                           │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  Frontend (HTML/CSS/JavaScript)                          │   │
│  │  - UI Components (Cards, Modal, Summary)                 │   │
│  │  - Event Handlers                                        │   │
│  │  - API Communication Layer                               │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                            ↕ HTTP/REST
┌─────────────────────────────────────────────────────────────────┐
│                      Backend Server (Go)                        │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  HTTP Router                                             │   │
│  │  - Request Multiplexing                                  │   │
│  │  - CORS Handling                                         │   │
│  │  - Static File Serving                                   │   │
│  └──────────────────────────────────────────────────────────┘   │
│                            ↓                                     │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  API Handlers                                            │   │
│  │  - List Computers                                        │   │
│  │  - Add Computer                                          │   │
│  │  - Ping Single                                           │   │
│  │  - Ping All                                              │   │
│  └──────────────────────────────────────────────────────────┘   │
│                            ↓                                     │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  Network Utilities                                       │   │
│  │  - TCP Connection Checker (Port 22)                      │   │
│  │  - Status Determination                                  │   │
│  └──────────────────────────────────────────────────────────┘   │
│                            ↓                                     │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  In-Memory Store                                         │   │
│  │  - Computer Registry                                     │   │
│  │  - Status Cache                                          │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                            ↕
┌─────────────────────────────────────────────────────────────────┐
│                    Network (TCP Port 22)                        │
│                    Target Computers (SSH)                       │
└─────────────────────────────────────────────────────────────────┘
```

## Component Description

### Frontend Components

#### Application Shell (`index.html`)
- Main HTML document structure
- Layout with header, summary section, and grid

#### User Interface (`styles.css`)
- Responsive CSS grid layout
- Dark theme with color-coded status indicators
- Smooth animations and transitions
- Mobile-friendly design

#### Client Logic (`app.js`)
- **API Service Layer**: Fetch wrapper functions for backend communication
- **State Management**: In-memory computer list
- **Event Handlers**: User interaction processing
- **Rendering Engine**: Dynamic card and UI generation
- **Modal Management**: Add computer dialog

### Backend Components

#### Router (`main.go::router`)
- HTTP multiplexer routing requests to handlers
- CORS header configuration
- Static file serving for frontend
- Method validation

#### API Handlers
- **listComputers**: Get all computers
- **addComputer**: Register new computer with validation
- **pingOne**: Check single computer status
- **pingAll**: Batch check all computers

#### Network Utilities
- **pingHost**: TCP connection test to port 22
- Timeout: 2 seconds per connection attempt
- Binary response: ON/OFF

#### Status Model
```go
type ComputerStatus struct {
    ID        string    // Computer identifier
    Name      string    // Display name
    IP        string    // IP address
    Status    string    // "ON" or "OFF"
    CheckedAt string    // Timestamp of check
}
```

## Data Flow

### Check Single Computer Flow

```
User clicks "Check" button
            ↓
Disable button, show spinner
            ↓
Client: pingOne(id)
            ↓
Server: GET /api/ping/{id}
            ↓
Find computer by ID
            ↓
TCP connection to port 22
            ↓
Determine status (ON/OFF)
            ↓
Return ComputerStatus with timestamp
            ↓
Update card UI with new status
            ↓
Re-enable button, hide spinner
```

### Add Computer Flow

```
User fills form and submits
            ↓
Validate all fields present
            ↓
Client: addComputer(data)
            ↓
Server: POST /api/computers
            ↓
Validate JSON body
            ↓
Check for required fields
            ↓
Check for duplicate ID
            ↓
Store in computers list
            ↓
Return created Computer object
            ↓
Add new card to UI
            ↓
Reset form, show success toast
```

## Security Considerations

### Frontend Security
- **XSS Prevention**: HTML escaping of user input before rendering
- **Input Validation**: Client-side form validation
- **HTTPS Ready**: Works with SSL/TLS proxies

### Backend Security
- **CORS Enabled**: Allows cross-origin requests safely
- **Input Validation**: Validates all request data
- **Error Handling**: Generic error messages prevent information leakage
- **Safe JSON**: Uses Go's standard JSON encoder

### Network Security
- **Port 22 (SSH)**: Only tests SSH availability, no authentication
- **TCP Timeout**: 2-second timeout prevents hanging
- **No Credentials**: Never handles passwords or secrets

## Performance Characteristics

### Response Times
- **List Computers**: < 10ms (memory only)
- **Ping Single**: 2-2500ms (2s timeout + network latency)
- **Ping All**: 2-2500ms (parallel for N computers)
- **Add Computer**: < 50ms (memory only)

### Scalability Limits
- **In-Memory Storage**: Limited by available RAM
- **Current**: ~10,000 computers before concerns
- **Ping All**: Linear time complexity O(n)
- **Concurrent Requests**: Limited by Go goroutines (thousands)

## Future Architecture Improvements

### Database Integration
```
Replace:    var computers = []Computer{}
With:       Query-based storage (PostgreSQL, MongoDB)
Benefits:   Persistence, scalability, multi-instance support
```

### Async Processing
```
Current:    Synchronous ping operations
Future:     Background worker queue
Benefits:   Non-blocking UI, status streaming
```

### Caching Layer
```
Add:        Redis or in-memory cache
Purpose:    Cache recent ping results
Benefits:   Reduced network load, faster responses
```

### Real-time Updates
```
Current:    Request-response pattern
Future:     WebSocket connection
Benefits:   Live status updates, reduced polling
```

## Development Guidelines

### Code Organization
- Clean separation of concerns
- Single responsibility principle
- Comprehensive error handling
- Proper logging at appropriate levels

### Naming Conventions
- **Go**: CamelCase for struct fields, lowercase for functions
- **JavaScript**: camelCase for functions, UPPER_CASE for constants
- **HTML/CSS**: kebab-case for class and ID names

### Error Handling
- Errors are logged and returned with contextual information
- Generic error messages to clients for security
- Detailed logs for debugging

### Testing Strategy
- Unit tests for utility functions
- Integration tests for API endpoints
- Manual testing for UI interactions

## Deployment Considerations

### Environment Variables
```bash
PORT=8081              # Server port
FRONTEND_DIR=frontend  # Frontend directory
LOG_LEVEL=INFO         # Logging level
```

### System Requirements
- RAM: Minimum 64MB
- CPU: Single core sufficient for small deployments
- Disk: 50MB for application + logs
- Network: TCP 8081 inbound

### Monitoring
- Application logs to stdout
- Status codes indicate errors (4xx, 5xx)
- Response times can be monitored via logs

---

**Architecture Version**: 1.0  
**Last Updated**: February 26, 2026
