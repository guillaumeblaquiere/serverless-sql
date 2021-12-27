FROM golang:1.17 AS builder

# Set necessary environmet variables needed for our image
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Move to working directory /build
WORKDIR /build

# Copy and download dependency using go mod
COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy the code into the container
COPY . .

# Build the application
RUN go build -o serverless-db-proxy .

FROM scratch

COPY --from=builder /build/serverless-db-proxy /
COPY --from=builder /build/startup.sh /

ENTRYPOINT ["/serverless-db-proxy"]
