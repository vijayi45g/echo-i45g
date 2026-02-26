# Network PC Monitoring System

A professional network monitoring application that allows you to check the status of computers on your network. Monitor multiple PCs simultaneously, add new computers dynamically, and get real-time status updates.

## Features

- ✅ **Real-time Monitoring**: Check individual computers or all at once
- ✅ **Add Computers**: Dynamically add new computers by IP address, name, and ID
- ✅ **Status Dashboard**: View online/offline status with visual indicators
- ✅ **Summary Statistics**: Track total, online, and offline computer counts
- ✅ **Professional UI**: Modern dark theme with responsive design
- ✅ **RESTful API**: Clean backend API for extensibility

## Project Structure

```
network-monitoring/
├── backend/                 # Go backend server
│   └── main.go             # REST API server implementation
├── frontend/               # React-less frontend
│   ├── index.html          # Main HTML structure
│   ├── app.js              # Client-side JavaScript
│   └── styles.css          # Responsive styling
├── docs/                   # Documentation
│   ├── ARCHITECTURE.md     # System architecture
│   └── API.md              # API documentation
├── .gitignore              # Git ignore patterns
├── README.md               # Project overview
└── go.mod                  # Go module file
```

## Prerequisites

- **Go 1.16+** - Backend runtime
- **Modern Web Browser** - Firefox, Chrome, Safari, Edge
- **Network Access** - SSH port (22) accessible on target computers

## Quick Start

### 1. Clone the Repository

```bash
git clone <repository-url>
cd network-monitoring
```

### 2. Start the Backend Server

```bash
cd backend
go run main.go
```

The server will start at `http://localhost:8081`

Output:
```
========================================
Network PC Monitoring System - Started
Server running at http://localhost:8081
========================================
```

### 3. Open the Frontend

Open your browser and navigate to:
```
http://localhost:8081
```

## Usage

### Check Individual Computer Status

Click the **Check** button on any computer card to ping that specific PC. The status will update to show "ON" or "OFF".

### Check All Computers

Click the **Check All** button in the header to ping all computers simultaneously. The summary counts will update automatically.

### Add a New Computer

1. Click **+ Add Computer** button
2. Fill in the form:
   - **ID**: Unique identifier (e.g., "3")
   - **Name**: Computer display name (e.g., "Office PC")
   - **IP**: IP address (e.g., "192.168.68.110")
3. Click **Add Computer**

## API Endpoints

### Get All Computers
```
GET /api/computers
```

### Add a Computer
```
POST /api/computers
Content-Type: application/json

{
  "id": "3",
  "name": "Office PC",
  "ip": "192.168.68.110"
}
```

### Ping Single Computer
```
GET /api/ping/{id}
```

### Ping All Computers
```
GET /api/ping-all
```

## Configuration

### Backend Port
To change the server port, edit `backend/main.go`:

```go
port := ":8081"  // Change to desired port
```

### Default Computers
To add default computers, edit the `computers` variable in `backend/main.go`:

```go
var computers = []Computer{
    {ID: "1", Name: "Cabin 12", IP: "192.168.68.92"},
    {ID: "2", Name: "Your PC", IP: "192.168.68.101"},
}
```

### Frontend API URL
To change the backend API URL, edit `frontend/app.js`:

```javascript
const API_BASE_URL = "http://localhost:8081/api";
```

## Technical Stack

- **Backend**: Go with standard library HTTP server
- **Frontend**: Vanilla JavaScript (no frameworks)
- **Styling**: Pure CSS with responsive design
- **Architecture**: RESTful API with in-memory storage

## How It Works

1. **Ping Mechanism**: The system checks computer status by attempting a TCP connection to port 22 (SSH)
2. **Status Values**: 
   - **ON**: Computer is reachable
   - **OFF**: Computer is not reachable
   - **UNKNOWN**: Computer hasn't been checked yet
3. **Timestamps**: Each check records the exact time the status was last verified

## Development

### Project Organization

- **Clean Code**: Well-commented, professional Go and JavaScript
- **Error Handling**: Comprehensive error handling and logging
- **Validation**: Input validation on both client and server
- **Security**: XSS protection, CORS headers, input sanitization

### Logging

The backend logs all operations:

```
INFO: Logged-in user 1
WARNING: Attempt to add duplicate computer ID: 1
ERROR: Failed to decode computer data: invalid syntax
```

## Troubleshooting

### Server Won't Start
- Check if port 8081 is already in use
- Ensure Go is properly installed: `go version`

### Cannot Connect to Backend
- Verify the server is running
- Check the API URL in `frontend/app.js`
- Ensure CORS is properly configured

### Computers Show as "OFF"
- Verify the IP addresses are correct
- Check network connectivity
- Ensure SSH port (22) is accessible
- Try pinging manually: `ping <ip-address>`

## Future Enhancements

- [ ] Database persistence
- [ ] User authentication
- [ ] Computer groups and categories
- [ ] Historical status tracking
- [ ] Email/Slack notifications
- [ ] Web socket real-time updates
- [ ] Custom port configuration
- [ ] Delete computer functionality

## License

MIT License - Feel free to use this project for personal or commercial purposes.

## Support

For issues, questions, or contributions, please reach out to the development team.

---

**Created**: February 26, 2026  
**Version**: 1.0.0  
**Status**: Production Ready
