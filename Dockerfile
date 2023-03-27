FROM golang:1.20.1 AS builder
# Define build env
ENV GOOS linux
ENV CGO_ENABLED 0
# Add a work directory
WORKDIR /app
CMD pwd
# Cache and install dependencies
COPY ["go.mod", "./"]
COPY ["go.sum", "./"]
RUN go mod download
# Copy app files
COPY ["pkg/", "./pkg/"]
WORKDIR /app/cmd/server
COPY ["cmd/server/*.go", "./"]
# Build app
RUN go build -o kv-server

FROM alpine:3.17.2 as runner
COPY --from=builder /app/cmd/server/kv-server .
EXPOSE 7000
CMD ./kv-server