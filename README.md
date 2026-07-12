# Track 1 Agent - LABLAB HACk

## Overview
This is a highly optimized, resource-aware routing agent for Hackathon Track 1. It dynamically processes tasks and routes them through a cost-efficiency hierarchy:
1. **Deterministic Solvers**: Solves math and logic puzzles locally using static analyzers and Python execution (0 tokens).
2. **Local LLM Sidecar**: Processes suitable tasks (e.g., sentiment analysis, summarization, general Q&A) using a bundled, 4-bit quantized `qwen2.5:1.5b` model running on an embedded Ollama server (0 tokens). It uses a semantic field-matching and self-rated confidence pipeline (temperature 0.1) to ensure accuracy before accepting a local answer.
3. **Fireworks API Escalation**: Any task that fails local confidence checks or requires heavy reasoning (e.g., complex code generation) is escalated to the Fireworks API. It preferentially attempts Gemma models first (for bonus criteria) before falling back to Minimax and Kimi, automatically retrying with strict schema-validation reminders if the API returns malformed output.

The agent is specifically engineered to run comfortably within the **4GB RAM / 2 vCPU** grading environment constraint. It includes a container-startup warmup sequence to completely eliminate cold-start latency.

## Architecture Pipeline
- `internal/classify`: Categorizes incoming tasks.
- `internal/solvers`: Attempts zero-shot deterministic resolution.
- `internal/local`: Interfaces with the embedded Ollama sidecar, employing multi-sample semantic field matching and self-rated confidence checks.
- `internal/fireworks`: Manages remote API calls, Gemma preference tracking, and fallback logic.
- `internal/router`: Orchestrates the decision flow and schema validation.

## Environment Variables
The grading harness will automatically inject the following environment variables at runtime. If you are testing this manually, you must provide them:
- `FIREWORKS_API_KEY`: Your real Fireworks API key (do not use "api" or fallback models will return 401 Unauthorized).
- `FIREWORKS_BASE_URL`: The API endpoint (e.g., `https://api.fireworks.ai/inference/v1`).
- `ALLOWED_MODELS`: Comma-separated list of permitted remote models.

## Usage & Testing

### 1. Pull the Image
The image is compiled specifically for `linux/amd64`.
```bash
docker pull YOUR_DOCKERHUB_USERNAME/track1-agent:latest
```

### 2. Run the Container
Execute the agent, mapping your input and output directories to the container:
```bash
docker run --rm \
  --memory=4g --memory-swap=4g --cpus=2 \
  -v C:\temp\track1\input:/input \
  -v C:\temp\track1\output:/output \
  -e FIREWORKS_API_KEY="YOUR_API_KEY_HERE" \
  -e FIREWORKS_BASE_URL="https://api.fireworks.ai/inference/v1" \
  -e ALLOWED_MODELS="accounts/fireworks/models/minimax-m3,accounts/fireworks/models/kimi-k2p7-code,accounts/fireworks/models/gemma-4-31b-it,accounts/fireworks/models/gemma-4-26b-a4b-it,accounts/fireworks/models/gemma-4-31b-it-nvfp4" \
  YOUR_DOCKERHUB_USERNAME/track1-agent:latest
```

## Build Instructions (For Development)
To rebuild the image from source for the `linux/amd64` architecture target:
```bash
docker buildx build --platform linux/amd64 -t YOUR_DOCKERHUB_USERNAME/track1-agent:latest .
```
