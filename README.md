# go-config-manager

A robust, generic configuration manager for Go applications using [Viper](https://github.com/spf13/viper).
This project demonstrates safe management, validation, and restoration of YAML configuration files with automatic backup and corruption recovery.

## Features

- Generic config manager for any struct
- Automatic validation of required keys and types
- Periodic config health checks and auto-restore from backup
- Simulated config updates, corruption, and random key injection for testing
- Thread-safe access and updates

## Getting Started

### Prerequisites

- Go 1.18 or newer

### Installation

Clone the repository:

```sh
git clone https://github.com/dimalukas/go-config-manager.git
cd go-config-manager
```

Install dependencies:

```sh
go mod tidy
```

### Usage

Run the example app:

```sh
go run main.go
```

This will:

- Load and validate `config.yaml`
- Restore from `config.backup.yaml` if the config is invalid or corrupted
- Periodically print the config, simulate updates, inject random keys, and corrupt the config file to demonstrate recovery

## Project Structure

- `main.go`: Example usage and simulation routines
- `config_manager.go`: Generic config manager implementation
- `config.yaml`: Main configuration file
- `config.backup.yaml`: Backup configuration file
- `go.mod`, `go.sum`: Go module files

## License

MIT License
