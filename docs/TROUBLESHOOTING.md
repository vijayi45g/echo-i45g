# Troubleshooting Guide

## Issue: POST request not adding computers

**Status:** ✅ FIXED - Now saves to SQLite database

### What was wrong:
- Data was only stored in memory
- Adding a computer worked temporarily but data was lost on server restart
- No persistent storage

### What's fixed:
- Backend now uses SQLite database (`monitoring.db`)
- POST /api/computers saves data to database table
- Data persists between server restarts

---

## Issue: GET method not properly checking computer status

**Status:** ✅ FIXED - Now retrieves from database correctly

### What was wrong:
- GET /api/computers was reading from in-memory array
- Could cause inconsistencies if data wasn't properly synchronized

### What's fixed:
- All GET requests now retrieve from SQLite database
- Status checks (ping operations) work correctly
- ON/OFF status properly reflects network connectivity on port 22

---

## Common Issues & Solutions

### 1. Server won't start
**Error:** `Failed to initialize database`
**Solution:** 
```bash
cd backend
rm -f monitoring.db  # Delete corrupted database
go build -o server main.go database.go
./server
```

### 2. "Cannot reach server" error on frontend
**Solution:**
- Ensure backend is running: `./server` in backend directory
- Check server is on port 8081: `lsof -i :8081`
- Ensure frontend is accessing correct API URL: `http://localhost:8081/api`

### 3. Computer added but doesn't appear
**Solution:**
- Open browser DevTools (F12)
- Check Network tab for POST request status (should be 201)
- Check console for error messages
- Verify all three fields (ID, Name, IP) are filled

### 4. Ping always shows OFFLINE
**Note:** This is expected if:
- Target computer doesn't have SSH port 22 open
- Network firewall blocks TCP connections to port 22
- Target computer is actually offline

---

## Database Management

### View database contents
```bash
sqlite3 backend/monitoring.db
# In sqlite3 prompt:
SELECT * FROM computers;
```

### Reset database
```bash
cd backend
rm -f monitoring.db
./server  # Creates fresh database
```

### Backup database
```bash
cp backend/monitoring.db backend/monitoring.db.backup
```

---

## Testing Commands

### Add computer via curl
```bash
curl -X POST http://localhost:8081/api/computers \
  -H "Content-Type: application/json" \
  -d '{"id":"2","name":"TestPC","ip":"192.168.1.100"}'
```

### List all computers
```bash
curl http://localhost:8081/api/computers
```

### Ping specific computer
```bash
curl http://localhost:8081/api/ping/2
```

### Ping all computers
```bash
curl http://localhost:8081/api/ping-all
```

---

## Files Modified

| File | Change |
|------|--------|
| `go.mod` | Added SQLite driver dependency |
| `backend/main.go` | Updated to use database functions |
| **`backend/database.go`** | **NEW** - SQLite integration |
| **`backend/monitoring.db`** | **NEW** - SQLite database file |
| `frontend/app.js` | No changes needed (already correct) |
| `frontend/index.html` | No changes needed (forms working) |
