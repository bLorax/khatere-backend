# Khātere Backend

This repo holds the backend code for Khātere. Khātere is a photobook app.

Khātere is an alternative to Instagram. Use Khātere to share photos.

Khātere follows digital minimalism guides.

The backend code is written in Go.

## About This Repo

This repo gives these items:

- The API server code.
- The Dockerfile. Use this file to build a container image of the server.
- The Go module files (`go.mod` and `go.sum`).

## Requirements

Before you set up this repo, install these items:

- Go (version 1.x or later).
- Docker. Use Docker only if you want to run the server in a container.

## Setup

Follow these steps to set up the repo on your computer.

1. Clone the repo.
   ```
   git clone https://github.com/bLorax/khatere-backend.git
   ```
2. Go to the repo folder.
   ```
   cd khatere-backend
   ```
3. Download the Go modules.
   ```
   go mod download
   ```

## Run the Server

Follow one of these two methods to run the server.

### Method 1: Run With Go

Use this command to run the server directly.
```
go run main.go
```

### Method 2: Run With Docker

Follow these steps to run the server in a Docker container.

1. Build the image.
   ```
   docker build -t khatere-backend .
   ```
2. Run the container.
   ```
   docker run khatere-backend
   ```

## Project Structure

The repo has this structure:

- `main.go` — This file starts the server.
- `internal/` — This folder holds the internal application code.
- `vendor/` — This folder holds the vendored dependencies.
- `Dockerfile` — Use this file to build the container image.

## Status

This project is in development. The API and the structure can change.

## Contributing

Contact the repo owner before you send a pull request.
