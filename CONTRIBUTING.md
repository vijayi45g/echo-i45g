# Contributing Guidelines

Thank you for your interest in contributing to Network PC Monitoring System!

## Getting Started

### Prerequisites
- Go 1.21 or later
- Basic understanding of REST APIs
- Familiarity with HTML/CSS/JavaScript

### Setup Development Environment

```bash
# Clone the repository
git clone <repository-url>
cd network-monitoring

# Start the backend
cd backend
go run main.go

# In another terminal, open the frontend
# http://localhost:8081
```

## Code Style Guidelines

### Go Backend
```go
// Use clear variable names
computer := Computer{
    ID:   "1",
    Name: "Test PC",
    IP:   "192.168.68.100",
}

// Add comments for exported functions
// listComputers handles GET /api/computers
func listComputers(w http.ResponseWriter, r *http.Request) {
    // implementation
}

// Use proper error handling
if err != nil {
    log.Printf("ERROR: %v", err)
    return
}
```

### JavaScript Frontend
```javascript
// Use clear function names and JSDoc comments
/**
 * Renders or updates a computer card in the grid
 * @param {Object} computer - Computer object to display
 * @param {Object} statusData - Optional ComputerStatus object
 */
function renderCard(computer, statusData = null) {
    // implementation
}

// Avoid global variables, use const/let
const API_BASE_URL = "http://localhost:8081/api";

// Add error handling
try {
    const results = await pingAll();
} catch (error) {
    showToast(`Error: ${error.message}`, "error");
}
```

## Commit Messages

Use clear, descriptive commit messages:

```
feat: Add DELETE endpoint for removing computers
fix: Correct timeout value for ping operations
docs: Update API documentation with examples
refactor: Simplify card rendering logic
test: Add unit tests for pingHost function
```

## Pull Request Process

1. Create a new branch: `git checkout -b feature/your-feature-name`
2. Make your changes and commit with clear messages
3. Push to your fork: `git push origin feature/your-feature-name`
4. Submit a Pull Request with a clear description

## Testing

### Manual Testing Checklist

- [ ] Check individual computer status
- [ ] Check all computers status
- [ ] Add a new computer
- [ ] Verify error handling
- [ ] Test on mobile devices
- [ ] Verify API responses

### Automated Testing

```bash
cd backend
go test ./...
```

## Reporting Issues

When reporting bugs, please include:

1. **Description**: What doesn't work?
2. **Steps to Reproduce**: How to trigger the issue?
3. **Expected Behavior**: What should happen?
4. **Actual Behavior**: What actually happens?
5. **Environment**: OS, Go version, browser version
6. **Logs**: Any error messages or logs

## Feature Requests

When suggesting features:

1. **Use Case**: Why do you need this?
2. **Proposed Solution**: How should it work?
3. **Alternative Solutions**: Other options?
4. **Additional Context**: Any other details?

## Areas for Contribution

### Backend
- [ ] Database persistence
- [ ] Authentication/Authorization
- [ ] Performance optimization
- [ ] Additional monitoring metrics

### Frontend
- [ ] UI improvements
- [ ] Mobile responsiveness
- [ ] Accessibility enhancements
- [ ] Real-time updates with WebSockets

### Documentation
- [ ] Tutorial videos
- [ ] Deployment guides
- [ ] API client libraries
- [ ] Case studies

## Questions?

Feel free to open an issue or contact the development team for guidance.

---

Thank you for making Network PC Monitoring System better!
