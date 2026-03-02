# Quick Start Guide - Terminal Application

## ⚡ Quick Start (60 seconds)

### 1. Generate SSH Keys (if not already done)
```bash
ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519 -N ""
```

### 2. Configure Remote Computers
Add your SSH key to target computers:
```bash
ssh-copy-id -i ~/.ssh/id_ed25519.pub user@remote_ip
```

### 3. Start Application
```bash
cd /home/sysadmin007/Documents/i45g/pc-monitoring/network-monitoring
./run-app.sh
```

### 4. Open in Browser
```
http://localhost:8081
```

### 5. Click Terminal Button on Any Computer Card

---

## UI Changes at a Glance

### Card Buttons - Before vs After

**Before** (Horizontal - crowded):
```
┌─────────────────────────────────────────┐
│ Computer Name              [Check] [Edit] [Delete] [Terminal] │
└─────────────────────────────────────────┘
```

**After** (Vertical - clean):
```
┌─────────────────────────────────────────┐
│ Computer Name                           │
│                                         │
│ ┌─────────────────────────────────────┐ │
│ │        [Terminal] (Green)           │ │
│ │        [Check] (Blue)               │ │
│ │        [Edit] (Blue)                │ │
│ │        [Delete] (Red)               │ │
│ └─────────────────────────────────────┘ │
└─────────────────────────────────────────┘
```

### Terminal - Before vs After

**Before** (Modal window):
```
┌─────────────────────────────────┐
│  Terminal - Computer (IP)    [X] │
├─────────────────────────────────┤
│                                 │
│  $ Connected to Computer...     │
│  $ Type 'help' for commands     │
│                                 │
│ (scrollable output area)        │
│                                 │
├─────────────────────────────────┤
│ $ [input field] [send]          │
│ 💡 Type exit to close           │
└─────────────────────────────────┘
```

**After** (Full-screen):
```
╔════════════════════════════════════════════════════════════════╗
║ ⌘ Computer Name (IP)                                      [X]  ║
╠════════════════════════════════════════════════════════════════╣
║                                                                ║
║ $ Connected to Computer at IP                                  ║
║ $ Type 'help' for commands or 'exit' to disconnect             ║
║                                                                ║
║ (Full-screen output area with better visibility)              ║
║                                                                ║
║                                                                ║
║                                                                ║
╠════════════════════════════════════════════════════════════════╣
║ $ [full-width input field] [send]                              ║
├────────────────────────────────────────────────────────────────┤
║ Ready        (status bar with connection indicator)            ║
╚════════════════════════════════════════════════════════════════╝
```

---

## Features Summary

### 🔐 SSH Improvements
| Feature | Before | After |
|---------|--------|-------|
| Key Support | RSA, ED25519 | RSA, ED25519, ECDSA |
| Key Detection | Basic | Enhanced + SSH Agent |
| Error Messages | Generic | Detailed diagnostics |
| Connection Timeout | 5s | 10s |
| Authentication | Single key | Multiple keys + agent |

### 📱 UI/UX Changes
| Element | Before | After |
|---------|--------|-------|
| Button Layout | Horizontal | Vertical |
| Button Size | Large (0.75rem) | Small (0.65rem) |
| Direction | Side-by-side | Down-by-down |
| Spacing | Fixed 0.6rem | Responsive 0.5rem |
| Responsiveness | Variable | Consistent |

### 🖥️ Terminal Interface
| Feature | Before | After |
|---------|--------|-------|
| Display Mode | Modal | Full-screen |
| Width | Max 900px | 100vw |
| Height | Max 80vh | 100vh |
| Header | Simple | Professional |
| Status Bar | None | Yes (Live) |
| Cursor | Hidden | Visible (Blinking) |
| Scrollbar | Standard | Gradient |

---

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| <kbd>Enter</kbd> | Send terminal command |
| <kbd>↑ Up Arrow</kbd> | Previous command |
| <kbd>↓ Down Arrow</kbd> | Next command |
| <kbd>Esc</kbd> | Close terminal |
| <kbd>Ctrl+A</kbd> | Select all (input) |
| <kbd>Ctrl+C</kbd> | Cancel running command* |

*Note: Some shortcuts depend on the remote system

---

## Command Examples

```bash
# List files with details
ls -lah

# Check disk usage
df -h

# View running processes
ps aux

# Check system info
uname -a

# Ping a host
ping -c 4 8.8.8.8

# View file contents
cat /etc/hostname

# Check current user and permissions
whoami && id

# Show command history
history

# Clear terminal (system command)
clear

# Download a file (note: terminal only, not direct download)
wget https://example.com/file.zip
```

---

## System Requirements

### Backend
- Go 1.20+
- SSH server running on target computers
- Port 22 (SSH) accessible from backend server
- Linux/Unix/macOS system

### Frontend
- Modern web browser (Chrome, Firefox, Safari, Edge)
- JavaScript enabled
- 1024x768 minimum resolution recommended

### SSH Configuration
- SSH keys (.ssh/id_rsa, id_ed25519, or id_ecdsa)
- Key permissions: 600 (read/write for owner only)
- SSH user account available on target systems
- Passwordless SSH auth or SSH key setup

---

## Troubleshooting

### Terminal Won't Open
**Problem**: "No computer selected" error  
**Solution**: Make sure computer is in the database. Click a computer card first.

### SSH Connection Fails
**Problem**: "SSH connection failed"  
```
Check:
1. Network connectivity: ping <ip>
2. SSH port: telnet <ip> 22
3. SSH server: ssh user@<ip> (manual test)
4. Firewall: sudo ufw status
5. SSH keys: ls -la ~/.ssh/
```

### Commands Not Executing
**Problem**: "command execution failed"  
```
Try:
1. Simple commands first: whoami, pwd, date
2. Check permissions on remote system
3. Some commands need special setup (docker, sudo, etc.)
4. Check remote system for command availability
```

### Buttons Overlapping on Mobile
**Problem**: Buttons stacked but look cramped  
**Solution**: This is normal. Mobile CSS automatically optimizes. Try landscape mode.

---

## Browser Console

If you encounter issues, check the browser console (<kbd>F12</kbd>) for error messages. Common errors:

```javascript
// Cannot reach server
"Failed to load computers: ... Cannot reach server"
// Fix: Start backend with ./run-app.sh

// Command failed
"Error: SSH execution failed: connection refused"
// Fix: Check SSH server is running and port 22 is open

// Invalid API response
"Error: SSH execution failed: SSH session failed"
// Fix: Check SSH key permissions and target system setup
```

---

## Files Modified

### Backend
- ✅ `backend/main.go` - Enhanced SSH authentication (350+ lines of SSH handling)
- ✅ `backend/go.mod` - Added crypto/ssh dependencies
- ✅ `backend/database.go` - No changes (kept stable)

### Frontend
- ✅ `frontend/index.html` - Full-screen terminal modal
- ✅ `frontend/app.js` - Terminal functions + event listeners
- ✅ `frontend/styles.css` - Terminal styling + button layout

### Scripts
- ✅ `run-app.sh` - Quick start launcher

---

## Performance Notes

### Terminal Output
- Large outputs (>1000 lines) may scroll slowly
- Recommended: Commands that output <500 lines for best experience
- Clear terminal if it gets cluttered: use `clear` command

### Network
- SSH timeout: 10 seconds
- Command execution time: depends on remote system
- Large file transfers: not recommended (use SCP instead)

### Browser Memory
- Terminal keeps output in memory (can grow large)
- Close terminal after use to free memory
- Reload page if terminal becomes slow

---

## Tips & Tricks

### Useful Commands
```bash
# Monitor system in real-time
top -n 1

# Check Apache/Nginx logs
tail -f /var/log/apache2/access.log
tail -f /var/log/nginx/access.log

# Check system resources
free -h          # Memory
df -h             # Disk space
vmstat 1 3        # Virtual memory

# Package management
apt update && apt list --upgradable  # Debian/Ubuntu
yum updates       # RHEL/CentOS

# User management
id                # Show current user
sudo -l           # Show sudo permissions
```

### Pro Tips
1. **Use command history** (↑/↓) to avoid retyping
2. **Test commands locally first** before running on production
3. **Use `echo` for quick checks**: `echo $HOME`
4. **Clear old output**: `type clear`
5. **Pipe commands**: `ls | grep pattern`

---

## Support

For issues or questions:
1. Check [TERMINAL_UPDATE.md](TERMINAL_UPDATE.md) for complete documentation
2. Review [TROUBLESHOOTING.md](TROUBLESHOOTING.md) for common issues
3. Check application logs: `backend/monitoring.db` database
4. Browser console errors: Press <kbd>F12</kbd>

---

**Last Updated**: February 27, 2026  
**Version**: 2.0 - Full-Screen Terminal  
**Status**: ✅ Production Ready
