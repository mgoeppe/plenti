[![CI](https://github.com/mgoeppe/plenti/actions/workflows/ci.yml/badge.svg)](https://github.com/mgoeppe/plenti/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/mgoeppe/plenti)](https://goreportcard.com/report/github.com/mgoeppe/plenti)
[![License](https://img.shields.io/github/license/mgoeppe/plenti)](LICENSE)

# Plenti <!-- omit from toc -->

Plenti is a command-line tool for interacting with Kostal Plenticore inverters. It allows you to retrieve, display, and store solar power data from your Plenticore inverter through its API.

- [Features](#features)
- [Installation](#installation)
  - [From Source](#from-source)
- [Configuration](#configuration)
  - [Example Configuration](#example-configuration)
- [Usage](#usage)
  - [Basic Commands](#basic-commands)
    - [List Available Fields](#list-available-fields)
    - [Display Current Data](#display-current-data)
    - [Save Data to Database](#save-data-to-database)
    - [Continuous Data Collection](#continuous-data-collection)
  - [Command Line Options](#command-line-options)
- [Docker Usage](#docker-usage)
  - [Building the Docker Image](#building-the-docker-image)
  - [Running the Container](#running-the-container)
  - [Docker Volumes](#docker-volumes)


## Features

- Connect to Kostal Plenticore inverter API securely
- Retrieve real-time data from the inverter
- List all available data fields
- Save data to a SQLite database at configurable intervals
- Configure data collection through a simple YAML file
- Dockerized deployment option for continuous data collection

## Installation

### From Source

1. Clone the repository:
   ```bash
   git clone https://github.com/mgoeppe/plenti.git
   cd plenti
   ```

2. Build the application:
   ```bash
   go build -o plenti
   ```

3. Move the binary to your PATH (optional):
   ```bash
   sudo mv plenti /usr/local/bin/
   ```

## Configuration

Plenti uses a YAML configuration file (`plenti.yaml`) for its settings. Create this file in the directory where you run Plenti, or specify its location with the `--config-path` flag.

### Example Configuration

```yaml
plenticore:
  server: "inverter.local"  # Your Plenticore inverter address
  password: "yourpassword"  # Your Plenticore password
  fields:                   # List of fields to collect (optional)
    - devices:local/Grid_P
    - devices:local/Home_P
    - devices:local:battery/SoC
    # Add more fields as needed
database:
  path: "plenti.db"        # Path to SQLite database file
  printSummary: true       # Print summary after saving data
```

You can generate a list of all available fields using the `fields` command.

## Usage

### Basic Commands

#### List Available Fields

```bash
plenti fields
```

This will list all available data fields from your Plenticore inverter, which you can then use in your configuration file.

#### Display Current Data

```bash
plenti data
```

This will display the current values for all configured fields (or all available fields if none are specified in the config).

#### Save Data to Database

```bash
plenti save
```

This will retrieve the current data and save it to the database specified in your configuration.

#### Continuous Data Collection

```bash
plenti save --interval=5m
```

This will save data to the database every 5 minutes. Supported formats include "30s", "5m", "1h", etc.

### Command Line Options

```
Global Flags:
  -c, --config-path string   Path to directory containing plenti.yaml config file (default ".")
      --log-level string     Log level (debug, info, warn, error) (default "info")
  -p, --password string      Plenticore password
  -s, --server string        Plenticore server address

Save Command Flags:
  -d, --database string      Database file path (default "plenti.db")
  -i, --interval string      Run continuously with specified interval (e.g. '30s', '5m', '1h')
      --summary              Print a summary after saving
```

## Docker Usage

For continuous data collection, you can use the provided Docker image.

### Building the Docker Image

```bash
docker build -t plenti-save .
```

### Running the Container

```bash
docker run -d \
  --name plenti-save \
  -v /path/to/your/config:/config \
  -v plenti-data:/data \
  plenti-save
```

### Docker Volumes

- `/config`: Mount directory containing your `plenti.yaml` configuration file
- `/data`: Mount point for the database storage

Make sure to configure all settings (including database path and interval) in your `plenti.yaml` file within the mounted config directory. For example:

```yaml
plenticore:
  server: "inverter.local"
  password: "yourpassword"
database:
  path: "/data/plenti.db"  # Note the path points to the data volume
  interval: "5m"           # Save data every 5 minutes
  printSummary: true
```
