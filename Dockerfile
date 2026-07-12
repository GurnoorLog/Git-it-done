# Build stage
FROM golang:1.25 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o track1-agent .

# Runtime stage
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    python3 \
    wget \
    unzip \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Download pre-built llama-server binary
RUN wget -q https://github.com/ggerganov/llama.cpp/releases/download/b4607/llama-b4607-bin-ubuntu-x64.zip -O /tmp/llama.zip && \
    unzip /tmp/llama.zip -d /tmp/llama && \
    cp /tmp/llama/build/bin/llama-server /usr/local/bin/llama-server && \
    rm -rf /tmp/llama.zip /tmp/llama

# Download Gemma 2 2B GGUF model (Q4_K_M)
RUN wget -q https://huggingface.co/bartowski/gemma-2-2b-it-GGUF/resolve/main/gemma-2-2b-it-Q4_K_M.gguf -O /app/model.gguf

COPY --from=builder /app/track1-agent .
RUN chmod +x /app/track1-agent

ENV LOCAL_MODEL_PATH=/app/model.gguf
CMD ["/app/track1-agent"]
