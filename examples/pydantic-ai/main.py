"""Pydantic AI — Type-safe agent with tools via Anthropic.

Run: ANTHROPIC_BASE_URL=http://127.0.0.1:8639 python main.py
Deps: pip install pydantic-ai anthropic
"""
import asyncio, os
from dataclasses import dataclass

os.environ.setdefault("ANTHROPIC_BASE_URL", "http://127.0.0.1:8639")
os.environ.setdefault("ANTHROPIC_API_KEY", "proxy-injected")

from pydantic_ai import Agent, RunContext

MODEL = "anthropic:claude-haiku-4-5-20251001"


@dataclass
class CityDB:
    cities: dict[str, dict]


db = CityDB(cities={
    "Tokyo": {"country": "Japan", "pop": "14M", "tz": "JST"},
    "Paris": {"country": "France", "pop": "2.1M", "tz": "CET"},
    "New York": {"country": "USA", "pop": "8.3M", "tz": "EST"},
})

agent = Agent(MODEL, deps_type=CityDB,
    system_prompt="You are a city info assistant. Use tools to look up data.")


@agent.tool
async def lookup_city(ctx: RunContext[CityDB], city: str) -> str:
    """Look up information about a city."""
    info = ctx.deps.cities.get(city)
    return f"{city}: {info['country']}, pop. {info['pop']}, {info['tz']}" if info else f"No data for {city}"


@agent.tool
async def list_cities(ctx: RunContext[CityDB]) -> str:
    """List available cities."""
    return ", ".join(ctx.deps.cities.keys())


async def main():
    result = await agent.run("What cities do you know about? Tell me about Tokyo and Paris.", deps=db)
    print(f"Agent: {result.output}")

    for msg in result.all_messages():
        if hasattr(msg, "parts"):
            for part in msg.parts:
                if hasattr(part, "tool_name"):
                    print(f"  [tool] {part.tool_name}")


if __name__ == "__main__":
    asyncio.run(main())
