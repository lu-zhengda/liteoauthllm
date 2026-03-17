"""Anthropic — Messages API with streaming and tool-call agentic loop.

Run: ANTHROPIC_BASE_URL=http://127.0.0.1:8639 python main.py
Deps: pip install anthropic
"""
import json, os

os.environ.setdefault("ANTHROPIC_BASE_URL", "http://127.0.0.1:8639")
os.environ.setdefault("ANTHROPIC_API_KEY", "proxy-injected")

import anthropic

client = anthropic.Anthropic()
MODEL = "claude-haiku-4-5-20251001"

tools = [{
    "name": "get_stock_price",
    "description": "Get current stock price for a ticker symbol",
    "input_schema": {
        "type": "object",
        "properties": {"ticker": {"type": "string"}},
        "required": ["ticker"],
    },
}]


def get_stock_price(ticker: str) -> dict:
    return {"AAPL": 187.50, "GOOGL": 141.80, "MSFT": 378.90}.get(ticker.upper(), 0)


def run(user_message: str):
    print(f"User: {user_message}")
    messages = [{"role": "user", "content": user_message}]

    while True:
        resp = client.messages.create(model=MODEL, max_tokens=500, messages=messages, tools=tools)

        if resp.stop_reason == "tool_use":
            messages.append({"role": "assistant", "content": resp.content})
            results = []
            for tb in [b for b in resp.content if b.type == "tool_use"]:
                price = get_stock_price(tb.input["ticker"])
                print(f"  [tool] {tb.name}({tb.input}) → ${price}")
                results.append({"type": "tool_result", "tool_use_id": tb.id, "content": json.dumps({"price": price})})
            messages.append({"role": "user", "content": results})
        else:
            for b in resp.content:
                if b.type == "text":
                    print(f"Assistant: {b.text}")
            break

    # Streaming demo
    print("\n--- Streaming ---")
    with client.messages.stream(model=MODEL, max_tokens=100,
            messages=[{"role": "user", "content": "Name 3 benefits of Go for proxies."}]) as stream:
        for text in stream.text_stream:
            print(text, end="", flush=True)
    print()


if __name__ == "__main__":
    run("Compare AAPL and GOOGL stock prices")
