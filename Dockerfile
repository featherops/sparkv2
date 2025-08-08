# syntax=docker/dockerfile:1

# Build web assets
FROM node:18-alpine AS web
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
# Build production bundle without relying on Windows-specific npm scripts
RUN NODE_ENV=production npx webpack --mode production

# Build Go server and embed web assets
FROM golang:1.18-alpine AS server
WORKDIR /app
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /app/web/dist ./web/dist
RUN go install github.com/rakyll/statik@v0.1.7
RUN statik -m -src="./web/dist" -f -dest="./server/embed" -p web -ns web
# Build static Linux binary
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -o spark-server ./server

# Prebuild client templates required by generator
RUN mkdir -p built \
    && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-H=windowsgui" -o built/windows_amd64 ./client \
    && CGO_ENABLED=0 GOOS=windows GOARCH=386   go build -ldflags "-H=windowsgui" -o built/windows_i386  ./client \
    && CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -o built/linux_amd64   ./client \
    && CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -o built/linux_arm64   ./client \
    && CGO_ENABLED=0 GOOS=linux   GOARCH=arm GOARM=7 go build -o built/linux_arm ./client

# Runtime image
FROM alpine:3.19
WORKDIR /app
RUN adduser -D -H spark
COPY --from=server /app/spark-server /app/spark-server
COPY --from=server /app/server/embed /app/server/embed
COPY --from=server /app/built /app/built

# Minimal config.json (listen will be overridden by PORT at runtime)
ARG SPARK_SALT=123456abcdef123456
ARG SPARK_LOG_LEVEL=info
RUN echo '{"listen":":8000","salt":"'${SPARK_SALT}'","auth":{},"log":{"level":"'${SPARK_LOG_LEVEL}'","path":"./logs","days":7}}' > /app/config.json

# Ensure writable directory for logs and ownership
RUN mkdir -p /app/logs && chown -R spark:spark /app

# Hugging Face Spaces sets PORT (e.g., 7860). Our server reads PORT and binds to it.
ENV PORT=7860 \
    SPARK_AUTH_USER=admin \
    SPARK_AUTH_PASS=admin \
    SPARK_LOG_LEVEL=info
EXPOSE 7860

USER spark
CMD ["/app/spark-server", "-config", "/app/config.json"]

