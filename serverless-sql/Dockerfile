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
RUN go build -o main .

FROM mysql:8

# Default ARCHIVE name in Cloud Storage.
ENV ARCHIVE_NAME="mysql-archive.tar"

RUN apt-get update -y && \
    apt-get install curl -y; \
    echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] http://packages.cloud.google.com/apt cloud-sdk main" | \
    tee -a /etc/apt/sources.list.d/google-cloud-sdk.list && \
    curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | \
    apt-key --keyring /usr/share/keyrings/cloud.google.gpg  add - && \
    apt-get update -y && \
    apt-get install google-cloud-sdk dumb-init -y


COPY --from=builder /build/main /

COPY --from=builder /build/startup.sh /

RUN chmod +x /startup.sh

ENTRYPOINT ["/usr/bin/dumb-init", "--"]

CMD  ["/startup.sh"]