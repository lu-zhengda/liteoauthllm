# liteoauthllm Examples

Real-world framework examples using liteoauthllm as the AI gateway proxy.

## Prerequisites

```bash
liteoauthllm login openai      # Browser OAuth
liteoauthllm login anthropic   # Paste setup-token
liteoauthllm                   # Start proxy on :8639
```

## Examples

| Example | Provider | Language |
|---------|----------|----------|
| `openai-py` | OpenAI (Codex Responses API) | Python |
| `openai-ts` | OpenAI (Codex Responses API) | TypeScript |
| `anthropic-py` | Anthropic (Messages API) | Python |
| `anthropic-ts` | Anthropic (Messages API) | TypeScript |
| `pydantic-ai` | Anthropic | Python |
| `langchain-py` | Anthropic | Python |
| `langchain-ts` | Anthropic | TypeScript |
| `vercel-ai` | Anthropic | TypeScript |

> **Note:** OpenAI Codex OAuth only supports the Responses API with Codex-specific models (e.g. `gpt-5.2-codex`). Most frameworks default to Chat Completions, so Anthropic is the more broadly compatible provider.
