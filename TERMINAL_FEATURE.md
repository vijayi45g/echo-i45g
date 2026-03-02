# Terminal Feature Documentation

## Overview

The Terminal feature allows you to execute commands directly on monitored computers via SSH. Each computer card now includes a **Terminal** button that opens an interactive terminal interface with a modern UI/UX.

## Features

### Frontend Features
- **Terminal Button**: Green button on each computer card to open the terminal
- **Modern Terminal UI**: Dark-themed terminal panel with scrollable output
- **Command History**: Navigate through previous commands using ↑/↓ arrow keys
- **Real-time Output**: See command results instantly
- **Colored Output**: Different colors for prompts, input, output, and errors
- **Auto-focus**: Input field automatically focused when terminal opens
- **Help Command**: Built-in help showing available commands

### Backend Features
- **SSH Connection**: Uses SSH keys for secure authentication
- **Key Management**: Automatically detects SSH keys from standard locations:
  - `~/.ssh/id_rsa`
  - `~/.ssh/id_ed25519`
  - `/root/.ssh/id_rsa`
  - `/home/sysadmin007/.ssh/id_rsa`
- **Command Execution**: Executes remote commands with output capture
- **Error Handling**: Graceful error messages if connection fails
- **CORS Support**: Works with cross-origin requests

## Usage

### Opening Terminal

1. Click the **Terminal** button (green, shows terminal icon) on any computer card
2. The terminal modal opens showing the computer name and IP address
3. You'll see a welcome message with instructions

### Executing Commands

1. Type a command in the input field at the bottom
2. Press **Enter** or click the send button (arrow icon)
3. Command output appears in the terminal window
4. Use **↑/↓ arrow keys** to navigate command history

### Available Commands

You can execute any command available on the remote system, including:
- `ls` - List directory contents
- `pwd` - Print working directory
- `whoami` - Show current user
- `date` - Show current date/time
- `uname -a` - Show system information
- `help` - Show help message
- `exit` or `quit` - Close terminal

### Closing Terminal

Click the **✕** button in the top-right corner of the terminal modal, or type `exit`/`quit`.

## Technical Details

### Frontend Files Modified
- **[index.html](frontend/index.html)**: Added terminal modal structure
- **[app.js](frontend/app.js)**: Added terminal functionality
  - `openTerminalModal()` - Open terminal for a computer
  - `closeTerminalModal()` - Close terminal
  - `sendTerminalCommand()` - Execute command
  - `addTerminalLine()` - Display output
  - `handleTerminalKeydown()` - Handle keyboard input
- **[styles.css](frontend/styles.css)**: Added terminal styling

### Backend Files Modified
- **[main.go](backend/main.go)**: 
  - Added SSH execution functions
  - `getSSHConfig()` - Load SSH private key
  - `executeSSHCommand()` - Execute command via SSH
  - `executeTerminal()` - HTTP handler for terminal requests
  - Added route: `POST /api/terminal/execute`
- **[go.mod](backend/go.mod)**: Added `golang.org/x/crypto` dependency

### API Endpoint

**POST /api/terminal/execute**

Request:
```json
{
  "computerId": "1",
  "command": "ls -la",
  "username": "root"  // optional, defaults to "root"
}
```

Response (Success):
```json
{
  "success": true,
  "data": {
    "output": "total 48\ndrwxr-xr-x  5 root root  4096 Feb 27 10:30 .\ndrwxr-xr-x 12 root root  4096 Feb 20 14:15 ..\n..."
  }
}
```

Response (Error):
```json
{
  "success": false,
  "error": "SSH execution failed: connection refused"
}
```

## Requirements

### SSH Keys Setup
The system requires SSH keys to be configured. Ensure:

1. **SSH Keys Exist**: Private key files in standard locations
2. **Correct Permissions**: Private keys should have `600` permissions
3. **Key Format**: Standard OpenSSH format (PKCS8 or RSA)
4. **Network Access**: SSH port (22) must be accessible on target computers

### Example SSH Key Setup
```bash
# On remote computers, ensure SSH server is running
ssh-keyscan -t rsa <computer_ip> >> ~/.ssh/known_hosts

# Copy public key to remote (if not using passwordless auth)
ssh-copy-id -i ~/.ssh/id_rsa.pub user@<computer_ip>
```

## Security Considerations

⚠️ **Important**: This feature should only be used on **trusted internal networks**!

### Current Limitations
- Session doesn't persist across page reloads
- No real-time bidirectional communication (stateless HTTP)
- SSH key must be accessible to the backend service
- No session timeout or activity logging

### Recommended Improvements
1. Implement WebSocket for persistent connections
2. Add command ACL/whitelist
3. Log all executed commands
4. Add session timeout
5. Implement multi-factor authentication
6. Add audit trail

## Troubleshooting

### "No SSH key found"
- Ensure SSH key exists in one of the standard paths
- Check key file permissions: `chmod 600 ~/.ssh/id_rsa`

### "Connection refused"
- Verify SSH port (22) is open on target computer
- Check SSH server is running: `systemctl status ssh`
- Verify IP address is correct

### Command execution fails
- Some commands require specific permissions or TTY
- Try simple commands first (e.g., `whoami`, `pwd`)
- Check the error message for specific details

### Terminal output is empty
- Command may have produced no output
- Some commands require TTY; not all work over SSH
- Check for error messages in the response

## Building and Running

### Build Backend
```bash
cd backend
go mod tidy
go build -o server main.go database.go
```

### Run Application
```bash
# Terminal 1: Run backend
cd backend
./server

# Terminal 2: Open frontend
# Open http://localhost:8081 in browser
```

## Future Enhancements

1. WebSocket support for persistent connections
2. Terminal session persistence
3. File upload/download
4. Multi-command queuing
5. Command output filtering
6. Interactive shell (bash, sh, etc.)
7. SCP support
8. Port forwarding
9. X11 forwarding
10. SFTP integration
