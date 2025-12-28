# API Diff Checker

A powerful tool to compare API responses across different versions. Execute curl commands against multiple API endpoints and instantly see the differences in their JSON responses.

![API Diff Checker](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)
![Platform](https://img.shields.io/badge/Platform-Windows%20|%20macOS%20|%20Linux-lightgrey)

## Features

- 🔄 **Multi-Version Comparison** - Compare API responses across any number of versions
- 📊 **Side-by-Side Diff View** - Visual split view showing old vs new JSON content
- 🎨 **Syntax Highlighting** - Color-coded JSON for easy reading
- 🔑 **Keys-Only Mode** - Compare only JSON structure (keys), ignoring values
- 📝 **Unified Diff** - Traditional git-style diff output
- 💾 **Response Storage** - All responses saved with timestamps for history
- 🌐 **Web Interface** - Modern, dark-themed UI
- 💻 **CLI Support** - Run from command line with config files

## Quick Start

### Prerequisites

- **Go 1.21+** installed ([Download Go](https://go.dev/dl/))
- **curl** available in your system PATH (pre-installed on most systems)

### Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/api_diff_checker.git
cd api_diff_checker

# Install dependencies
go mod download
```

### Running the Web Interface

#### Windows

```powershell
# Build
go build -o api_diff_checker.exe .

# Run web server
.\api_diff_checker.exe --web
```

#### macOS / Linux

```bash
# Build
go build -o api_diff_checker .

# Run web server
./api_diff_checker --web
```

Then open your browser at **http://localhost:9876**

### Running from CLI

Create a config file `config.json`:

```json
{
  "versions": {
    "prod": "https://api.example.com",
    "staging": "https://staging-api.example.com"
  },
  "commands": ["curl {{BASE_URL}}/users/1", "curl {{BASE_URL}}/posts?limit=10"],
  "keys_only": false
}
```

Run the comparison:

#### Windows

```powershell
.\api_diff_checker.exe config.json
```

#### macOS / Linux

```bash
./api_diff_checker config.json
```

## Usage Guide

### Web Interface

1. **Add Versions** - Enter a name (e.g., `v1`, `prod`) and the base URL for each API version
2. **Add Commands** - Enter curl commands using `{{BASE_URL}}` as a placeholder
3. **Toggle Keys-Only** - Enable to compare only JSON structure, ignoring values
4. **Run Comparison** - Click the button to execute and compare

### Command Placeholder

Use `{{BASE_URL}}` in your curl commands. It will be replaced with each version's URL:

```
curl {{BASE_URL}}/api/users -H "Authorization: Bearer token123"
```

This becomes:

- `curl https://api.example.com/api/users -H "Authorization: Bearer token123"` (for prod)
- `curl https://staging.example.com/api/users -H "Authorization: Bearer token123"` (for staging)

### Keys-Only Mode

When enabled, the comparison ignores actual values and only checks if the JSON structure matches:

**Full Comparison:**

```json
{"name": "John", "age": 30}  vs  {"name": "Jane", "age": 25}
// Result: Differences Found (values differ)
```

**Keys-Only Comparison:**

```json
{"name": "John", "age": 30}  vs  {"name": "Jane", "age": 25}
// Result: Match (same structure)
```

## Project Structure

```
api_diff_checker/
├── main.go              # Entry point (CLI/Web mode selector)
├── config/
│   └── config.go        # Configuration parsing
├── core/
│   └── engine.go        # Main execution engine
├── executor/
│   └── runner.go        # Curl command executor
├── comparator/
│   └── diff.go          # JSON comparison logic
├── storage/
│   └── store.go         # Response storage
├── logger/
│   └── log.go           # Logging utilities
├── server/
│   └── server.go        # HTTP server
├── static/
│   ├── index.html       # Web UI
│   ├── style.css        # Styles
│   └── app.js           # Frontend logic
└── responses/           # Saved API responses
```

## Building from Source

### All Platforms

```bash
# Download dependencies
go mod download

# Build for current platform
go build -o api_diff_checker .
```

### Cross-Compilation

```bash
# Windows (from any platform)
GOOS=windows GOARCH=amd64 go build -o api_diff_checker.exe .

# macOS Intel
GOOS=darwin GOARCH=amd64 go build -o api_diff_checker_mac .

# macOS Apple Silicon
GOOS=darwin GOARCH=arm64 go build -o api_diff_checker_mac_arm .

# Linux
GOOS=linux GOARCH=amd64 go build -o api_diff_checker_linux .
```

## Output

### Response Files

All API responses are saved in the `responses/` directory:

- Format: `v{version}_{command-hash}_{timestamp}.json`
- An `index.json` file tracks all executions

### Logs

Execution logs are saved to `execution.log` with timestamps and error details.

## API Reference (Web Server)

### `POST /api/run`

Execute comparison with the provided configuration.

**Request Body:**

```json
{
  "versions": {
    "v1": "https://api1.example.com",
    "v2": "https://api2.example.com"
  },
  "commands": ["curl {{BASE_URL}}/endpoint"],
  "keys_only": false
}
```

**Response:**

```json
{
  "command_results": [
    {
      "command": "curl ...",
      "diffs": [
        {
          "version_a": "v1",
          "version_b": "v2",
          "diff_result": {
            "text_diff": "...",
            "summary": "Field 'status' changed"
          },
          "old_content": "{...}",
          "new_content": "{...}"
        }
      ],
      "execution_info": [...]
    }
  ]
}
```

## Troubleshooting

### "curl: command not found"

Install curl:

- **Windows**: Download from [curl.se](https://curl.se/windows/) or use `winget install curl`
- **macOS**: Pre-installed, or `brew install curl`
- **Linux**: `sudo apt install curl` or `sudo yum install curl`

### Port 9876 already in use

Another application is using port 9876. Stop it or modify the server code to use a different port.

### SSL Certificate errors

If comparing HTTPS endpoints with self-signed certificates, add `-k` to your curl commands:

```
curl -k {{BASE_URL}}/api/endpoint
```

## License

MIT License - feel free to use and modify as needed.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
