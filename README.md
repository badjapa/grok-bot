# Grok Bot

A Discord bot powered by Grok AI that provides intelligent conversational capabilities.

## Features

- Discord bot integration using discordgo
- Grok AI API integration for intelligent responses
- Chat history management
- Environment variable configuration
- Cross-platform builds (Windows, Linux, macOS)

## Prerequisites

- Go 1.25.3 or later
- Discord Bot Token
- Grok API Key

## Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd grok-bot
```

2. Install dependencies:
```bash
make deps
```

3. Set up environment variables:
```bash
make env-setup
```

4. Edit the `.env` file with your actual values:
```env
DISCORD_TOKEN=your_discord_token_here
GROK_API_KEY=your_grok_api_key_here
```

## Usage

### Building the Project

```bash
# Build for Windows (default)
make build

# Build for all platforms
make build-all

# Build for specific platform
make build-windows
make build-linux
make build-mac
```

### Running the Bot

```bash
# Run with environment variables from env.bat
make run-env

# Or set up environment and run
make env-setup
make run
```

### Development

```bash
# Development mode with auto-reload (requires air)
make install-air
make dev

# Format code
make fmt

# Run tests
make test

# Run tests with coverage
make test-coverage
```

### Code Quality

```bash
# Lint code (requires golangci-lint)
make install-lint
make lint

# Format code
make fmt
```

### Dependency Management

```bash
# Install dependencies
make deps

# Update dependencies
make update-deps
```

### Utility Commands

```bash
# Clean build artifacts
make clean

# Show all available commands
make help

# Set up environment template
make env-setup
```

## Makefile Commands

| Command | Description |
|---------|-------------|
| `make all` | Clean, install deps, and build |
| `make build` | Build the application |
| `make build-all` | Build for Windows, Linux, and macOS |
| `make build-windows` | Build for Windows only |
| `make build-linux` | Build for Linux only |
| `make build-mac` | Build for macOS only |
| `make clean` | Clean build artifacts |
| `make run` | Build and run the application |
| `make run-env` | Set environment variables and run |
| `make env-setup` | Create .env template file |
| `make deps` | Install dependencies |
| `make update-deps` | Update dependencies |
| `make test` | Run tests |
| `make test-coverage` | Run tests with coverage report |
| `make fmt` | Format code |
| `make lint` | Lint code (requires golangci-lint) |
| `make install-lint` | Install golangci-lint |
| `make dev` | Development mode with auto-reload |
| `make install-air` | Install air for development |
| `make help` | Show all available commands |

## Project Structure

```
grok-bot/
├── bot/                 # Bot package
│   ├── bot.go          # Main bot logic
│   ├── grok_client.go  # Grok API client
│   ├── history.go      # Chat history management
│   └── util.go         # Utility functions
├── build/              # Build artifacts (created by make)
├── tmp/                # Temporary files (created by make)
├── main.go             # Application entry point
├── go.mod              # Go module definition
├── go.sum              # Go module checksums
├── Makefile            # Build automation
├── env.bat             # Environment variables batch file
└── README.md           # This file
```

## Environment Variables

The bot requires the following environment variables:

- `DISCORD_TOKEN`: Your Discord bot token
- `GROK_API_KEY`: Your Grok API key

You can set these using:
- The `.env` file (created by `make env-setup`)
- The `env.bat` file (Windows batch file)
- System environment variables

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `make test`
5. Format code: `make fmt`
6. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
