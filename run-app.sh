#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}================================================${NC}"
echo -e "${BLUE}  Network Monitoring Terminal - Quick Start${NC}"
echo -e "${BLUE}================================================${NC}"
echo ""

# Rebuild backend if binary is missing or source changed
if [ ! -f "./backend/server" ] || [ "./backend/main.go" -nt "./backend/server" ] || [ "./backend/database.go" -nt "./backend/server" ]; then
    echo -e "${YELLOW}⚠️  Backend binary missing/outdated. Building now...${NC}"
    cd backend
    go build -o server main.go database.go
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✅ Backend built successfully${NC}"
        cd ..
    else
        echo -e "${RED}❌ Failed to build backend${NC}"
        exit 1
    fi
fi

# Check if SSH keys exist
if [ ! -f "$HOME/.ssh/id_rsa" ] && [ ! -f "$HOME/.ssh/id_ed25519" ]; then
    echo -e "${YELLOW}⚠️  No SSH keys found!${NC}"
    echo -e "${YELLOW}   Terminal feature requires SSH keys for authentication${NC}"
    echo -e "${YELLOW}   Generate keys with: ssh-keygen -t rsa${NC}"
    echo ""
fi

echo -e "${BLUE}Starting server...${NC}"
cd backend
./server &
SERVER_PID=$!
echo -e "${GREEN}✅ Server started (PID: $SERVER_PID)${NC}"
echo ""

# Wait for server to start
sleep 2

# Check if server is running
if ! kill -0 $SERVER_PID 2>/dev/null; then
    echo -e "${RED}❌ Server failed to start${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Server is running!${NC}"
echo ""
echo -e "${BLUE}================================================${NC}"
echo -e "${BLUE}  Application Ready${NC}"
echo -e "${BLUE}================================================${NC}"
echo ""
echo -e "📱 ${YELLOW}Frontend:${NC}        http://localhost:8081"
echo -e "🔌 ${YELLOW}Backend API:${NC}     http://localhost:8081/api"
echo -e "🖥️  ${YELLOW}Terminal:${NC}         Click 'Terminal' button on any device"
echo -e "🔐 ${YELLOW}SSH Port:${NC}         22 (must be accessible)"
echo ""
echo -e "${YELLOW}Keyboard Shortcuts:${NC}"
echo -e "  • Enter              - Send command"
echo -e "  • ↑/↓ Arrow Keys     - Command history"
echo -e "  • ESC                - Close terminal"
echo ""
echo -e "${YELLOW}Stop server:${NC}   Press Ctrl+C"
echo ""

# Wait for user to stop the server
wait $SERVER_PID
