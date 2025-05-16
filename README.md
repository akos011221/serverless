# Serverless Platform

A serverless platform for deploying and executing functions in Docker containers with HTTP triggers.

## Features

- Deploy Go functions via CLI (`serverless deploy`).
- Invoke functions with JSON events (`serverless invoke` or HTTP POST).
- Run functions in isolated Docker containers.
- Store function metadata in SQLite.
- Structured JSON logging for observability.

## Prerequisites

- Go 1.23+
- Docker: Install with `sudo apt install docker.io` (Ubuntu) or `brew install docker` (macOS).
- Docker daemon running: `sudo systemctl start docker`.

## Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/akos011221/serverless
   cd serverless-platform

2. Install dependencies:
    ```bash
    go mod tidy

3. Build the CLI:
    ```bash
    go build -o serverless ./cmd/serverless

## Usage

1. Start the server in aterminal:
   ```bash
   ./serverless run --config config/config.yaml

2. Deploy the example function
   ```bash
   ./serverless deploy example

