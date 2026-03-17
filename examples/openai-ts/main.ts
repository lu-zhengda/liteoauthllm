/**
 * OpenAI Codex — Responses API with tool-call agentic loop.
 *
 * Run: OPENAI_BASE_URL=http://127.0.0.1:8639/v1 npx tsx main.ts
 * Deps: npm install openai
 */
import OpenAI from "openai";

const client = new OpenAI({
  baseURL: process.env.OPENAI_BASE_URL || "http://127.0.0.1:8639/v1",
  apiKey: "proxy-injected",
});

const MODEL = "gpt-5.2-codex";

const tools: OpenAI.Responses.Tool[] = [{
  type: "function", name: "get_weather",
  description: "Get current weather for a city",
  parameters: { type: "object", properties: { city: { type: "string" } }, required: ["city"] },
}];

function getWeather(city: string) {
  return { Tokyo: { temp: "22°C", condition: "sunny" }, London: { temp: "14°C", condition: "cloudy" } }[city]
    ?? { temp: "unknown", condition: "unknown" };
}

async function main() {
  console.log("User: What's the weather in Tokyo?");
  const input: OpenAI.Responses.ResponseInput = [{ role: "user", content: "What's the weather in Tokyo?" }];

  // First call (streaming required by Codex)
  let events = [];
  for await (const e of await client.responses.create({
    model: MODEL, instructions: "Use the get_weather tool.", input, tools, store: false, stream: true,
  })) events.push(e);

  const resp = events.find((e) => e.type === "response.completed")!;
  if (resp.type !== "response.completed") return;

  for (const tc of resp.response.output.filter((o): o is OpenAI.Responses.ResponseFunctionToolCall => o.type === "function_call")) {
    const args = JSON.parse(tc.arguments);
    const result = getWeather(args.city);
    console.log(`  [tool] ${tc.name}(${JSON.stringify(args)}) → ${JSON.stringify(result)}`);
    input.push(tc, { type: "function_call_output", call_id: tc.call_id, output: JSON.stringify(result) });
  }

  // Second call
  events = [];
  for await (const e of await client.responses.create({
    model: MODEL, instructions: "You are helpful.", input, tools, store: false, stream: true,
  })) events.push(e);

  const resp2 = events.find((e) => e.type === "response.completed");
  if (resp2?.type === "response.completed") {
    for (const item of resp2.response.output)
      if (item.type === "message") for (const c of item.content)
        if (c.type === "output_text") console.log(`Assistant: ${c.text}`);
  }
}

main().catch(console.error);
