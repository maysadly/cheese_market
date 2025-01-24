# -------------------------------------------------
# Stage 1: Build the Go application
# -------------------------------------------------
FROM golang:bookworm AS builder

WORKDIR /app

# Copy go.mod and go.sum first to cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Now copy the rest of the source code
COPY . .

# (Optional) If you use a vendor folder
# RUN go mod vendor

# Build your Go application
RUN go build -o main .

# -------------------------------------------------
# Stage 2: Create a minimal runtime image
# -------------------------------------------------
FROM debian:bookworm-slim

WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder /app /app

# Expose the application port (optional)
EXPOSE 8080

# You can set the default command here (instead of in docker-compose)
# CMD ["./main"]