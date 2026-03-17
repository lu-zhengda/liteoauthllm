# liteoauthllm

[![CI](https://github.com/lu-zhengda/liteoauthllm/actions/workflows/ci.yml/badge.svg)](https://github.com/lu-zhengda/liteoauthllm/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/lu-zhengda/liteoauthllm)](https://goreportcard.com/report/github.com/lu-zhengda/liteoauthllm)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A lightweight local AI gateway proxy that uses ChatGPT or Claude subscription OAuth to proxy requests to OpenAI and Anthropic APIs.

## Quick Start

```bash
go install github.com/lu-zhengda/liteoauthllm/cmd/liteoauthllm@latest

liteoauthllm login openai        # Opens browser for OAuth
liteoauthllm login anthropic     # Paste token from `claude setup-token`
liteoauthllm                     # Start proxy on :8639

export OPENAI_BASE_URL=http://127.0.0.1:8639/v1
export ANTHROPIC_BASE_URL=http://127.0.0.1:8639
```

See [`examples/`](examples/) for Pydantic AI, LangChain, Vercel AI SDK, and more.

## How It Works

Routes requests by path, injects auth headers, streams responses back unmodified.

| Path | Provider | Notes |
|------|----------|-------|
| `/v1/responses` | OpenAI | Codex models only (e.g. `gpt-5.3-codex`) |
| `/v1/messages` | Anthropic | All subscription models |
| `/v1/models` | Auto-detect | `anthropic-version` header → Anthropic, otherwise OpenAI |

## Legal

This project is **not** affiliated with OpenAI or Anthropic. For **personal, local use only**. You are solely responsible for compliance with provider terms of service. Provided "as is" with no warranty.

## License

[MIT](LICENSE)
