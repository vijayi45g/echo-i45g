#!/bin/bash

# ============================================================================
# Network PC Monitoring System - Git Initialization Script
# ============================================================================
# Run this script to initialize git version control for the project
# Usage: chmod +x setup-git.sh && ./setup-git.sh
# ============================================================================

echo "=========================================="
echo "Git Repository Initialization"
echo "=========================================="

# Change to project directory
cd "$(dirname "$0")" || exit 1

# Initialize git repository
if [ ! -d ".git" ]; then
    echo "Initializing git repository..."
    git init
    echo "✓ Git repository initialized"
else
    echo "✓ Git repository already exists"
fi

# Configure git user (optional)
read -p "Configure git user? (y/n): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    read -p "Enter your name: " GIT_USER_NAME
    read -p "Enter your email: " GIT_USER_EMAIL
    git config user.name "$GIT_USER_NAME"
    git config user.email "$GIT_USER_EMAIL"
    echo "✓ Git user configured locally"
fi

# Create .gitignore (already done, but verify)
if [ -f ".gitignore" ]; then
    echo "✓ .gitignore file exists"
else
    echo "✗ .gitignore file not found"
fi

# Add all files
echo ""
echo "Adding files to git..."
git add -A
echo "✓ Files added to staging area"

# Show status
echo ""
echo "Repository Status:"
git status

# Create initial commit
echo ""
read -p "Create initial commit? (y/n): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    git commit -m "Initial commit: Professional Network PC Monitoring System

- Clean code structure with proper comments
- RESTful API backend in Go
- Responsive frontend with vanilla JavaScript
- Comprehensive documentation
- Professional project layout"
    echo "✓ Initial commit created"
else
    echo "Skipped initial commit"
fi

# Show git log
echo ""
echo "Repository initialized! Here's the history:"
git log --oneline

echo ""
echo "=========================================="
echo "✓ Git setup complete!"
echo "=========================================="
echo ""
echo "Next steps:"
echo "1. Create a remote repository on GitHub/GitLab"
echo "2. Add remote: git remote add origin <repository-url>"
echo "3. Push: git push -u origin main"
echo ""
