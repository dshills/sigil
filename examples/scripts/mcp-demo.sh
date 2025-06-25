#!/bin/bash
# MCP (Model Context Protocol) Demonstration Script
# This script shows how to work with MCP servers in Sigil

set -e

echo "🚀 Sigil MCP Demonstration"
echo "=========================="
echo

# Check if sigil binary exists
if ! command -v sigil &> /dev/null; then
    echo "❌ Sigil binary not found. Please build it first:"
    echo "   go build -o sigil cmd/sigil/main.go"
    exit 1
fi

echo "📋 Available MCP Commands:"
echo "1. List configured MCP servers"
echo "2. Show MCP server status"
echo "3. Start an MCP server"
echo "4. Stop an MCP server"
echo "5. Use MCP model for code explanation"
echo

# Function to run command with nice output
run_command() {
    echo "▶️  Running: $1"
    echo "────────────────────────────────────"
    eval "$1"
    echo
}

echo "🔍 Step 1: List configured MCP servers"
run_command "sigil mcp list"

echo "📊 Step 2: Check MCP server status"
run_command "sigil mcp status"

echo "🎯 Step 3: Initialize MCP configuration (if needed)"
if [ ! -f ".sigil/mcp-servers.yml" ]; then
    echo "Creating example MCP configuration..."
    run_command "sigil mcp init"
else
    echo "✅ MCP configuration already exists"
fi
echo

# Check if GitHub token is available for demo
if [ -n "$GITHUB_TOKEN" ]; then
    echo "🐙 Step 4: Start GitHub MCP server (token found)"
    run_command "sigil mcp start github-mcp || echo 'Server may already be running or configuration needs adjustment'"
    
    echo "🔍 Step 5: Use GitHub MCP for code analysis"
    echo "Creating a sample file to analyze..."
    cat > sample_code.go << 'EOF'
package main

import (
    "fmt"
    "net/http"
    "log"
)

func main() {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "Hello, World!")
    })
    
    log.Println("Server starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
EOF

    echo "🤖 Explaining code using MCP model..."
    run_command "sigil explain sample_code.go --model 'mcp://github-mcp/default' --format markdown || echo 'MCP model not available, using default model'"
    
    echo "🧹 Cleaning up..."
    rm -f sample_code.go
    
else
    echo "⚠️  Step 4: GitHub token not found (set GITHUB_TOKEN to enable GitHub MCP demo)"
fi

echo "📚 Step 6: MCP Resource Management"
echo "MCP servers can provide resources (files, data, APIs). Here's how to interact with them:"
echo
echo "Example commands you can try:"
echo "  sigil ask --model 'mcp://server-name/model' 'What tools are available?'"
echo "  sigil explain myfile.py --model 'mcp://python-tools/assistant'"
echo "  sigil edit database.sql --model 'mcp://postgres-mcp/default'"
echo

echo "🔧 Step 7: MCP Health Monitoring"
run_command "sigil mcp status --json | jq '.servers[] | {name: .name, status: .status, uptime: .uptime}' || sigil mcp status"

echo "✅ MCP Demonstration Complete!"
echo
echo "🎓 What you learned:"
echo "  • How to configure MCP servers"
echo "  • How to start/stop MCP servers"
echo "  • How to use MCP models in Sigil commands"
echo "  • How to monitor MCP server health"
echo
echo "📖 For more information:"
echo "  • See examples/config/mcp-servers.yaml for configuration examples"
echo "  • Run 'sigil mcp --help' for detailed command help"
echo "  • Check the MCP documentation at https://modelcontextprotocol.io"
echo