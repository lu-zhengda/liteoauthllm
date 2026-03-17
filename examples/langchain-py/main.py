"""LangChain — ReAct agent with tools via Anthropic.

Run: ANTHROPIC_BASE_URL=http://127.0.0.1:8639 python main.py
Deps: pip install langchain langchain-anthropic
"""
import os

os.environ.setdefault("ANTHROPIC_BASE_URL", "http://127.0.0.1:8639")
os.environ.setdefault("ANTHROPIC_API_KEY", "proxy-injected")

from langchain_anthropic import ChatAnthropic
from langchain_core.messages import HumanMessage, ToolMessage
from langchain_core.tools import tool

MODEL = "claude-haiku-4-5-20251001"


@tool
def calculate(expression: str) -> str:
    """Evaluate a math expression using Python syntax."""
    try:
        return str(eval(expression, {"__builtins__": {}}, {}))
    except Exception as e:
        return f"Error: {e}"


@tool
def unit_convert(value: float, from_unit: str, to_unit: str) -> str:
    """Convert between units (km/miles, kg/lbs, celsius/fahrenheit)."""
    conversions = {
        ("km", "miles"): 0.621371, ("miles", "km"): 1.60934,
        ("kg", "lbs"): 2.20462, ("lbs", "kg"): 0.453592,
        ("celsius", "fahrenheit"): lambda v: v * 9 / 5 + 32,
        ("fahrenheit", "celsius"): lambda v: (v - 32) * 5 / 9,
    }
    f = conversions.get((from_unit.lower(), to_unit.lower()))
    if f is None: return f"Unknown: {from_unit} → {to_unit}"
    result = f(value) if callable(f) else value * f
    return f"{value} {from_unit} = {result:.2f} {to_unit}"


def main():
    llm = ChatAnthropic(model=MODEL, max_tokens=500).bind_tools([calculate, unit_convert])
    messages = [HumanMessage(content="What is 15 * 37? Also convert 100 km to miles.")]
    print(f"User: {messages[0].content}")

    while True:
        resp = llm.invoke(messages)
        messages.append(resp)

        if resp.tool_calls:
            for tc in resp.tool_calls:
                fn = {"calculate": calculate, "unit_convert": unit_convert}[tc["name"]]
                result = fn.invoke(tc["args"])
                print(f"  [tool] {tc['name']}({tc['args']}) → {result}")
                messages.append(ToolMessage(content=str(result), tool_call_id=tc["id"]))
        else:
            print(f"Assistant: {resp.content}")
            break


if __name__ == "__main__":
    main()
