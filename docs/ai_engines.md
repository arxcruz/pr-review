# 🤖 AI Providers & Endpoints Configuration

`pr-review` supports a flexible **many-to-many** AI configuration model:
- **Multiple Models per Provider**: Define a list of models for a single provider.
- **Multiple Endpoints / Instances**: Configure multiple separate instances/servers running Ollama, vLLM, llama.cpp, or cloud endpoints using `ai.endpoints`.
- **Project Context Awareness**: Include repository descriptions so AI models understand the architectural context of the code they review.

---

## ⚙️ Configuration Schema

### 1. Standard Providers with Multiple Models

```yaml
default_ai_provider: "ollama"

ai:
  # Ollama (Local or Remote)
  ollama:
    base_url: "http://localhost:11434"
    models:
      - "qwen2.5-coder:latest"
      - "deepseek-coder-v2:16b"
      - "llama3.3:70b"
    temperature: 0.2

  # vLLM (High-throughput GPU inference)
  vllm:
    base_url: "http://localhost:8000/v1"
    models:
      - "Qwen/Qwen2.5-Coder-32B-Instruct"
      - "deepseek-ai/DeepSeek-Coder-V2-Instruct"
    temperature: 0.2

  # llama.cpp (llama-server)
  llamacpp:
    base_url: "http://localhost:8080/v1"
    model: "default"
    temperature: 0.2

  # Anthropic Claude
  anthropic:
    api_key: "${ANTHROPIC_API_KEY}"
    models:
      - "claude-3-7-sonnet-20250219"
      - "claude-3-5-haiku-20241022"
    temperature: 0.2

  # Google Gemini
  gemini:
    api_key: "${GEMINI_API_KEY}"
    models:
      - "gemini-2.5-flash"
      - "gemini-2.5-pro"

  # OpenAI
  openai:
    api_key: "${OPENAI_API_KEY}"
    models:
      - "gpt-4o"
      - "o3-mini"
```

---

### 2. Multi-Endpoint Configurations (`ai.endpoints`)

If you have multiple servers running Ollama, vLLM, LM Studio, or local clusters, define them under `ai.endpoints`:

```yaml
ai:
  endpoints:
    # Dedicated vLLM GPU Server #1
    - id: "gpu1-vllm"
      name: "GPU 1 (DeepSeek Coder)"
      provider: "vllm"
      base_url: "http://192.168.1.50:8000/v1"
      api_key: "${VLLM_API_KEY}"
      models:
        - "deepseek-ai/DeepSeek-Coder-V2-Instruct"
      temperature: 0.2

    # Dedicated vLLM GPU Server #2
    - id: "gpu2-vllm"
      name: "GPU 2 (Qwen 32B)"
      provider: "vllm"
      base_url: "http://192.168.1.51:8000/v1"
      models:
        - "Qwen/Qwen2.5-Coder-32B-Instruct"
      temperature: 0.2

    # Remote Ollama Instance
    - id: "remote-ollama"
      name: "Office Ollama Cluster"
      provider: "ollama"
      base_url: "http://10.0.0.100:11434"
      models:
        - "qwen2.5-coder:32b"
        - "codellama:70b"

    # Local llama.cpp Instance
    - id: "local-llamacpp"
      name: "llama.cpp (Local Metal/CUDA)"
      provider: "llamacpp"
      base_url: "http://localhost:8080/v1"
      model: "default"
      temperature: 0.2
```

---

## 🏷️ Project Context Descriptions

Provide a `description` field for each configured project in `projects:`. This is included in the AI prompt to improve review accuracy:

```yaml
projects:
  - id: "backend"
    provider: "github"
    owner: "myorg"
    repo: "backend"
    description: "Core microservice backend built with Go, gRPC, and PostgreSQL. Follows clean architecture."
    default_filters:
      labels: ["needs-review"]

  - id: "frontend"
    provider: "gitlab"
    project_path: "myorg/web-frontend"
    description: "React and TypeScript web frontend using Next.js, Tailwind CSS, and TanStack Query."
```
