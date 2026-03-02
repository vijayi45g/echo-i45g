# 🚀 Network Monitoring System - Quick Start Guide

## Terminal Commands (Copy & Paste These)

### Step 1: Navigate to Backend Directory
```bash
cd ~/Documents/i45g/pc-monitoring/network-monitoring/backend
```

### Step 2: Build the Server
```bash
go build -o server main.go database.go
```
✅ This creates the `server` executable binary

### Step 3: Run the Server
```bash
./server
```

You should see:
```
2026/02/27 17:17:24 INFO: Database connected successfully at monitoring.db
2026/02/27 17:17:24 INFO: Database tables initialized
2026/02/27 17:17:24 ========================================
2026/02/27 17:17:24 Network PC Monitoring System - Started
2026/02/27 17:17:24 Server running at http://localhost:8081
2026/02/27 17:17:24 Database: monitoring.db
2026/02/27 17:17:24 ========================================
```

**DO NOT CLOSE THIS TERMINAL** - Keep it running while using the app

---

### Step 4: Open Your Browser
Open a new browser tab and go to:
```
http://localhost:8081
```

---

## Features Available

✅ **Check Single Computer** - Click "Check" button on any card  
✅ **Check All Computers** - Click "Check All" button in header  
✅ **Add Computer** - Click "+ Add Computer" to add new machines  
✅ **Edit Computer** - Click "Edit" button to update name/IP  
✅ **Delete Computer** - Click "Delete" button to remove from database  

---

## To Stop the Server

Press **Ctrl+C** in the terminal running the server

---

## Database & Storage

- **Type**: SQLite3
- **Location**: `backend/monitoring.db`
- **Data**: Persists between restarts
- **Size**: ~16KB (auto-grows as needed)

---

## Troubleshooting

### Port Already in Use
If you get "port already in use" error:
```bash
# Find process using port 8081
lsof -i :8081

# Kill the process (replace PID with actual number)
kill -9 PID
```

### Server Won't Start
```bash
# Rebuild the binary
go build -o server main.go database.go

# Then try running again
./server
```

### Browser Shows "Cannot Reach Server"
- Make sure terminal shows "Server running at http://localhost:8081"
- Make sure you haven't closed the terminal
- Try hard refresh: **Ctrl+Shift+R** (or Cmd+Shift+R on Mac)

---

## File Structure
```
backend/
  ├── main.go          (API endpoints & routing)
  ├── database.go      (SQLite database operations)
  ├── server           (compiled binary)
  └── monitoring.db    (SQLite database file)

frontend/
  ├── index.html       (UI structure)
  ├── app.js           (JavaScript logic)
  └── styles.css       (Modern UI styling)
```

---

**Ready to go!** 🎯 Run the commands above in your terminal now!
