FROM golang:1.24-alpine

WORKDIR /app

# Install air for hot reload
RUN go install github.com/air-verse/air@latest

# Copy go mod files first for better caching
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Expose backend port
EXPOSE 8081

CMD ["air", "-c", ".air.toml"]
