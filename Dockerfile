# Step 1: Builder stage
FROM golang:1.24.6-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application source code and build the application
COPY . .
RUN go build -o tunmanager -ldflags="-w -s" -trimpath -v .

# Step 2: Runner stage
FROM alpine:3.20

# Install necessary networking tools
RUN apk add --no-cache iproute2 iptables wireguard-tools

# Copy the compiled binary from the builder stage
COPY --from=builder /app/tunmanager /tunmanager

# Set the entrypoint to run the application
ENTRYPOINT ["/tunmanager"]
