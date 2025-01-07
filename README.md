# Cheese Market

## Project Overview
Cheese Market is a web application that manages a product catalog using a RESTful API with MongoDB as the database. It supports CRUD operations for products.

### Features:
- Add Products: Add new products with a name and price.
- View Products: Display all available products in the catalog.
- Update Products: Modify product details using the ID.
- Delete Products: Remove products by their ID.
- Static Files Support: Serves static resources like stylesheets and JavaScript files.

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
   git clone https://github.com/maysadly/cheese_market.git
   cd cheese_market/cheese_market
   ```
3. **Install dependencies:**
    ```bash
   go mod tidy
   ```
4. **Start MongoDB compass**
   
5. **Run the Server**:
   ```bash
   go run main.go
   ```
   The server will start and listen on `localhost:8080`.

---

## Tools and Resources Used
- **Go Programming Language**: For developing the HTTP server.
- **JSON**: To parse and validate payloads.
- **Postman**: For testing the API.
- **Visual Studio Code**: As the code editor.
- **Linux/Mac/Windows Terminal**: For running the server.
- **MongoDB**: For storing data.

---

## How to Start Task 1

1. **Install Go**:
   - Download and install Go from the official website: [https://golang.org/dl/](https://golang.org/dl/)

2. **Clone the Repository**:
   ```bash
   git clone https://github.com/maysadly/cheese_market.git
   cd cheese_market
   ```
3. **Install dependencies:**
    ```bash
   go mod tidy
   ```
4. **Run the Server**:
   ```bash
   go run main.go
   ```
   The server will start and listen on `localhost:8080`.

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
curl -X POST http://localhost:8080/ -H "Content-Type: application/json" -d '{"message1": "Hello, World!"}'
```
**Response**:
```json
{
  "status": "fail",
  "message": "Invalid JSON message"
}
```
