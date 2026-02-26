# API Documentation

## Base URL

```
http://localhost:8081/api
```

## Authentication

No authentication required for current version.

## Response Format

All API responses follow a consistent JSON structure:

```json
{
  "success": true,
  "data": {},
  "error": null
}
```

### Success Response
```json
{
  "success": true,
  "data": { /* resource data */ }
}
```

### Error Response
```json
{
  "success": false,
  "data": null,
  "error": "Error message describing what went wrong"
}
```

## Endpoints

---

## GET /api/computers

Retrieves all computers in the monitoring system.

### Request
```
GET /api/computers
```

### Response (200 OK)
```json
{
  "success": true,
  "data": [
    {
      "id": "1",
      "name": "Cabin 12",
      "ip": "192.168.68.92"
    },
    {
      "id": "2",
      "name": "Office PC",
      "ip": "192.168.68.110"
    }
  ]
}
```

### Example
```bash
curl http://localhost:8081/api/computers
```

---

## POST /api/computers

Adds a new computer to the monitoring system.

### Request
```
POST /api/computers
Content-Type: application/json

{
  "id": "3",
  "name": "Server Room PC",
  "ip": "192.168.68.101"
}
```

### Request Parameters
- `id` (string, required): Unique identifier for the computer
- `name` (string, required): Display name for the computer
- `ip` (string, required): IP address of the computer

### Response (201 Created)
```json
{
  "success": true,
  "data": {
    "id": "3",
    "name": "Server Room PC",
    "ip": "192.168.68.101"
  }
}
```

### Response (400 Bad Request)
```json
{
  "success": false,
  "error": "id, name and ip are required"
}
```

### Response (409 Conflict)
```json
{
  "success": false,
  "error": "Computer with this ID already exists"
}
```

### Example
```bash
curl -X POST http://localhost:8081/api/computers \
  -H "Content-Type: application/json" \
  -d '{
    "id": "3",
    "name": "Server Room PC",
    "ip": "192.168.68.101"
  }'
```

---

## GET /api/ping/{id}

Checks the status of a specific computer.

### Request
```
GET /api/ping/{id}
```

### Parameters
- `id` (string, path, required): Computer ID to check

### Response (200 OK)
```json
{
  "success": true,
  "data": {
    "id": "1",
    "name": "Cabin 12",
    "ip": "192.168.68.92",
    "status": "ON",
    "checkedAt": "2026-02-26 14:30:45"
  }
}
```

### Response (404 Not Found)
```json
{
  "success": false,
  "error": "Computer not found"
}
```

### Status Values
- `ON`: Computer is reachable (SSH port 22 is open)
- `OFF`: Computer is not reachable (SSH port 22 is closed or unreachable)

### Example
```bash
curl http://localhost:8081/api/ping/1
```

---

## GET /api/ping-all

Checks the status of all computers in the system.

### Request
```
GET /api/ping-all
```

### Response (200 OK)
```json
{
  "success": true,
  "data": [
    {
      "id": "1",
      "name": "Cabin 12",
      "ip": "192.168.68.92",
      "status": "ON",
      "checkedAt": "2026-02-26 14:30:45"
    },
    {
      "id": "2",
      "name": "Office PC",
      "ip": "192.168.68.110",
      "status": "OFF",
      "checkedAt": "2026-02-26 14:30:45"
    },
    {
      "id": "3",
      "name": "Server Room PC",
      "ip": "192.168.68.101",
      "status": "ON",
      "checkedAt": "2026-02-26 14:30:45"
    }
  ]
}
```

### Example
```bash
curl http://localhost:8081/api/ping-all
```

---

## Error Codes

| Code | Meaning | Description |
|------|---------|-------------|
| 200 | OK | Request successful |
| 201 | Created | Resource successfully created |
| 400 | Bad Request | Invalid request parameters or JSON body |
| 404 | Not Found | Requested resource not found |
| 409 | Conflict | Resource conflict (e.g., duplicate ID) |
| 500 | Internal Error | Server error (rare) |

---

## Rate Limiting

No rate limiting is currently implemented. For production, consider:

- Max 100 requests/minute per IP
- Max 10 concurrent connections
- Max 5-minute connection timeout

---

## CORS

The API includes CORS headers for browser requests:

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, OPTIONS
Access-Control-Allow-Headers: Content-Type
```

---

## Data Types

### Computer Object
```json
{
  "id": "string",          // Unique identifier
  "name": "string",        // Display name
  "ip": "string"           // IP address (IPv4)
}
```

### ComputerStatus Object
```json
{
  "id": "string",          // Computer ID
  "name": "string",        // Display name
  "ip": "string",          // IP address
  "status": "ON|OFF",      // Current status
  "checkedAt": "string"    // ISO timestamp
}
```

---

## Implementation Notes

### Ping Mechanism
- Uses TCP connection to port 22 (SSH)
- 2-second timeout per connection attempt
- Binary result: ON if successful, OFF if failed
- No actual SSH connection is established

### Timestamps
- Format: `YYYY-MM-DD HH:MM:SS` (24-hour format)
- Timezone: Server local time
- Updated on each ping operation

### Validation Rules
- `id`: Required, must be unique
- `name`: Required, non-empty string
- `ip`: Required, valid IPv4 format recommended

---

## Usage Examples

### JavaScript/Fetch

```javascript
// Get all computers
const response = await fetch('http://localhost:8081/api/computers');
const data = await response.json();
console.log(data.data);

// Add a computer
const newComputer = await fetch('http://localhost:8081/api/computers', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    id: '4',
    name: 'Test PC',
    ip: '192.168.68.105'
  })
});

// Ping all computers
const statuses = await fetch('http://localhost:8081/api/ping-all');
const statusData = await statuses.json();
console.log(statusData.data);
```

### Python/Requests

```python
import requests
import json

BASE_URL = 'http://localhost:8081/api'

# Get all computers
response = requests.get(f'{BASE_URL}/computers')
computers = response.json()['data']

# Add computer
new_computer = {
    'id': '4',
    'name': 'Test PC',
    'ip': '192.168.68.105'
}
response = requests.post(f'{BASE_URL}/computers', json=new_computer)
result = response.json()

# Ping all
response = requests.get(f'{BASE_URL}/ping-all')
statuses = response.json()['data']
```

### cURL

```bash
# Get all computers
curl http://localhost:8081/api/computers | jq

# Add computer
curl -X POST http://localhost:8081/api/computers \
  -H "Content-Type: application/json" \
  -d '{"id":"4", "name":"Test PC", "ip":"192.168.68.105"}' | jq

# Ping all
curl http://localhost:8081/api/ping-all | jq
```

---

**API Version**: 1.0.0  
**Last Updated**: February 26, 2026
