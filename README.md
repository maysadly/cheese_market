# Simple Go HTTP Server

## Project Overview
This project implements a simple HTTP server using Go. The server processes HTTP requests, validates JSON payloads, and provides appropriate responses based on the request method and data.

### Features:
- Accepts `GET` and `POST` requests.
- Validates JSON payloads for `POST` requests.
- Returns appropriate HTTP status codes and JSON responses for different scenarios.

---

## Purpose
The purpose of this project is to demonstrate a minimal yet functional HTTP server in Go. This server can be a starting point for more complex web applications or APIs.

---

## Team Members
1. Kamil
2. Olzhas
3. Tileukhan

---

## How to Start the Project

1. **Install Go**:
   - Download and install Go from the official website: [https://golang.org/dl/](https://golang.org/dl/)

2. **Clone the Repository**:
   ```bash
   git clone https://github.com/your-repo-name/simple-go-http-server.git
   cd simple-go-http-server
   ```

3. **Run the Server**:
   ```bash
   go run main.go
   ```
   The server will start and listen on `localhost:8080`.

4. **Send Requests**:
   - Use a tool like [Postman](https://www.postman.com/) or `curl` to test the server:
     ```bash
     curl -X POST http://localhost:8080/ -H "Content-Type: application/json" -d '{"message": "Hello, World!"}'
     ```

---

## Tools and Resources Used
- **Go Programming Language**: For developing the HTTP server.
- **JSON**: To parse and validate payloads.
- **Postman**: For testing the API.
- **Visual Studio Code**: As the code editor.
- **Linux/Mac/Windows Terminal**: For running the server.

---

## Example Usage

### Valid Request (POST)
**Request**:
```bash
curl -X POST http://localhost:8080/ -H "Content-Type: application/json" -d '{"message": "Hello, World!"}'
```

**Response**:
```json
{
  "status": "success",
  "message": "Data successfully received"
}
```

### Invalid Method (PUT)
**Request**:
```bash
curl -X PUT http://localhost:8080/
```

**Response**:
```json
{
  "status": "fail",
  "message": "Method not allowed. Only GET and POST are supported."
}
```

### Missing JSON Field
**Request**:
```bash
curl -X POST http://localhost:8080/ -H "Content-Type: application/json" -d '{}'
```

**Response**:
```json
{
  "status": "fail",
  "message": "Invalid JSON message"
}
```

---
