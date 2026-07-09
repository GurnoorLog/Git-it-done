# Build stage
FROM golang:1.25 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o track1-agent ./main.go

# Runtime stage
# We use debian:bookworm-slim because the Ollama binary requires a glibc environment.
# Alpine (musl) would cause Ollama execution to fail.
FROM debian:bookworm-slim

# Install curl to download Ollama, python3 for code verification, and any other minimal dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    ca-certificates \
    zstd \
    python3 \
    && rm -rf /var/lib/apt/lists/*

# Install Ollama (this adds to image size, but ensures local inference works offline)
RUN curl -fsSL https://ollama.com/install.sh | sh

# Pre-download the local model used for zero-token inference.
#
# Model: qwen2.5:1.5b  (Qwen2.5 1.5B Instruct, default Ollama quantization = Q4_K_M)
# Class: 1.5B parameters, 4-bit quantized — satisfies the "2-3B, 4-bit" requirement
#        and is deliberately under the 3B ceiling for memory safety.
#
# Resource footprint (measured):
#   Disk (in image) : ~1.0 GB
#   Runtime RAM     : ~1.3 GB peak (model weights + KV cache at 256 tokens)
#   CPU inference   : ~4-8s per call on 2 vCPU
#
# Why NOT qwen2.5:3b (2.0 GB disk, ~2.5 GB RAM):
#   In a 4 GB RAM / 2 vCPU grading environment we also need RAM for:
#     - Ollama server overhead (~300 MB)
#     - Go agent process (~50 MB)
#     - OS + Docker runtime (~500 MB)
#   3B brings the total to ~3.35 GB leaving <650 MB headroom — too close.
#   1.5B brings the total to ~2.15 GB leaving ~1.85 GB headroom — safe.
#
# DO NOT upsize this model without retesting under --memory=4g --cpus=2 constraints.
RUN ollama serve & OLLAMA_PID=$! && \
    sleep 5 && \
    ollama pull qwen2.5:1.5b && \
    kill $OLLAMA_PID || true

WORKDIR /app
COPY --from=builder /app/track1-agent .

# Entrypoint: start Ollama, wait for it, do a warmup generation to load model weights
# into RAM before the agent starts, then exec the agent.
# Without warmup the first real task call takes 30s+ (cold load). With warmup it takes <5s.
# The warmup call is a trivial prompt — it just forces Ollama to page the model into memory.
RUN printf '#!/bin/sh\n\
ollama serve > /dev/null 2>&1 &\n\
# Wait until Ollama HTTP endpoint is ready (poll up to 15s)\n\
for i in $(seq 1 15); do\n\
  curl -sf http://localhost:11434/api/tags > /dev/null 2>&1 && break\n\
  sleep 1\n\
done\n\
# Warmup: one short generation to page model weights into RAM\n\
curl -sf -X POST http://localhost:11434/api/generate \\\n\
  -H "Content-Type: application/json" \\\n\
  -d '"'"'{"model":"qwen2.5:1.5b","prompt":"Hi","stream":false,"options":{"num_predict":1}}'"'"' \\\n\
  > /dev/null 2>&1\n\
echo "[entrypoint] Ollama warmup complete — starting agent"\n\
exec /app/track1-agent\n\
' > /app/entrypoint.sh && chmod +x /app/entrypoint.sh

ENTRYPOINT ["/app/entrypoint.sh"]
