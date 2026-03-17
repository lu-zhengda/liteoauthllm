"""OpenAI Codex — Responses API with tool-call agentic loop."""
import json
import os

os.environ.setdefault("OPENAI_BASE_URL", "http://127.0.0.1:8639/v1")
os.environ.setdefault("OPENAI_API_KEY", "proxy-injected")

from openai import OpenAI

client = OpenAI()
MODEL = "gpt-5.2-codex"

tools = [{
    "type": "function",
    "name": "get_weather",
    "description": "Get current weather for a city",
    "parameters": {
        "type": "object",
        "properties": {"city": {"type": "string"}},
        "required": ["city"],
    },
}]


def get_weather(city: str) -> dict:
    return {
        "Tokyo": {"temp": "22°C", "condition": "sunny"},
        "London": {"temp": "14°C", "condition": "cloudy"},
    }.get(city, {"temp": "unknown", "condition": "unknown"})


def run(user_message: str):
    print(f"User: {user_message}")
    input_items = [{"role": "user", "content": user_message}]

    while True:
        events = list(client.responses.create(
            model=MODEL,
            instructions="You are a helpful weather assistant. Use the get_weather tool.",
            input=input_items, tools=tools, store=False, stream=True,
        ))

        response = next(e for e in events if e.type == "response.completed").response
        tool_calls = [o for o in response.output if o.type == "function_call"]

        if tool_calls:
            for tc in tool_calls:
                args = json.loads(tc.arguments)
                result = get_weather(args.get("city", ""))
                print(f"  [tool] {tc.name}({args}) → {result}")
                input_items.extend([tc, {
                    "type": "function_call_output",
                    "call_id": tc.call_id,
                    "output": json.dumps(result),
                }])
        else:
            for item in response.output:
                if item.type == "message":
                    for c in item.content:
                        if c.type == "output_text":
                            print(f"Assistant: {c.text}")
            break


if __name__ == "__main__":
    run("What's the weather in Tokyo and London?")
