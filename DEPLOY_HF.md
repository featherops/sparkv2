## Deploy to Hugging Face Spaces (Docker)

This project is ready to run on Hugging Face Spaces using Docker. You have two ways to deploy:

### Option A (Recommended): Connect Space to your GitHub repo
- Create a Space with runtime: Docker.
- Import or connect the Space to `featherops/sparkv2`.
- The Space will build from the `Dockerfile` in the repo root.
- Set Space Secrets (Settings → Variables & secrets):
  - `SPARK_AUTH_USER`: your admin username (required)
  - `SPARK_AUTH_PASS`: your admin password (required)
  - `SPARK_SALT`: short random salt (<=24 chars recommended) (optional but recommended)
  - `SPARK_LOG_LEVEL`: `disable|fatal|error|warn|info|debug` (optional)
  - Do NOT set `PORT` manually; Spaces sets it (e.g. 7860). The app auto-binds to `:$PORT`.

That’s it. Spaces will build and run the server at `https://<your-space>.hf.space` on the single exposed port.

Notes:
- Client generator/auto-update endpoints require prebuilt artifacts in `./built/<os>_<arch>`. If you don’t include those in the image, generator/update won’t work (other features are fine).
- Default `admin/admin` is set in the Dockerfile for convenience—override via secrets above.

### Option B: Only upload a Dockerfile (clone code during build)
If you prefer to only provide a Dockerfile to the Space (and not the entire repo), use the following Dockerfile. It clones the public repo during the build.

```
# syntax=docker/dockerfile:1

FROM golang:1.18-alpine AS build
WORKDIR /src
RUN apk add --no-cache git nodejs npm

# Set these if you want to point at a fork/branch
ARG REPO=https://github.com/featherops/sparkv2.git
ARG REF=main
RUN git clone --depth 1 --branch ${REF} ${REPO} .

# Build web assets
WORKDIR /src/web
RUN npm ci --no-audit --no-fund
RUN NODE_ENV=production npx webpack --mode production

# Embed web assets and build server
WORKDIR /src
RUN go install github.com/rakyll/statik@v0.1.7
RUN cp -r web/dist ./web/dist && \
    statik -m -src="./web/dist" -f -dest="./server/embed" -p web -ns web

ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go mod download
RUN go build -o spark-server ./server

FROM alpine:3.19
WORKDIR /app
RUN adduser -D -H spark
COPY --from=build /src/spark-server /app/spark-server
COPY --from=build /src/server/embed /app/server/embed

# Minimal config.json (overridden by env at runtime)
ENV PORT=7860 \
    SPARK_AUTH_USER=admin \
    SPARK_AUTH_PASS=admin \
    SPARK_LOG_LEVEL=info

RUN echo '{"listen":":8000","salt":"123456abcdef123456","auth":{},"log":{"level":"info","path":"./logs","days":7}}' > /app/config.json
RUN mkdir -p /app/logs && chown -R spark:spark /app

EXPOSE 7860
USER spark
CMD ["/app/spark-server", "-config", "/app/config.json"]
```

Secrets to add in the Space (both options)

| Name              | Required | Example            | Purpose                              |
|-------------------|----------|--------------------|--------------------------------------|
| `SPARK_AUTH_USER` | Yes      | `admin`            | Basic Auth username                  |
| `SPARK_AUTH_PASS` | Yes      | `supersecret`      | Basic Auth password                  |
| `SPARK_SALT`      | Recommended | `mysalthash123` | Server-side salt (<=24 chars)        |
| `SPARK_LOG_LEVEL` | Optional | `info`             | Log level                            |
| `PORT`            | No       | `7860`             | Set by Spaces automatically          |

The application automatically reads these at runtime. You do not need to edit or upload a `config.json`.

### Health checks & notes
- WebSockets are served on the same port and work behind the Spaces proxy.
- Logs are written to `/app/logs` inside the container (ephemeral on Spaces).
- If you want client generation/update features, add prebuilt client binaries into `./built` in your image or repository before build.

