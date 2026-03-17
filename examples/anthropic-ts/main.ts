/**
 * Anthropic — Messages API with streaming and tool-call loop.
 *
 * Run: ANTHROPIC_BASE_URL=http://127.0.0.1:8639 npx tsx main.ts
 * Deps: npm install @anthropic-ai/sdk
 */
import Anthropic from "@anthropic-ai/sdk";

const client = new Anthropic({
  baseURL: process.env.ANTHROPIC_BASE_URL || "http://127.0.0.1:8639",
  apiKey: "proxy-injected",
});

const MODEL = "claude-haiku-4-5-20251001";

const tools: Anthropic.Messages.Tool[] = [{
  name: "search_docs",
  description: "Search documentation for a topic",
  input_schema: { type: "object" as const, properties: { query: { type: "string" } }, required: ["query"] },
}];

function searchDocs(query: string): string {
  const docs: Record<string, string> = {
    proxy: "liteoauthllm routes requests to OpenAI and Anthropic APIs.",
    auth: "Uses PKCE OAuth for OpenAI and setup-tokens for Anthropic.",
  };
  return Object.entries(docs).find(([k]) => query.toLowerCase().includes(k))?.[1] ?? "Not found.";
}

async function main() {
  console.log("User: How does liteoauthllm handle auth?");
  const messages: Anthropic.Messages.MessageParam[] = [
    { role: "user", content: "How does liteoauthllm handle auth? Search the docs." },
  ];

  while (true) {
    const resp = await client.messages.create({ model: MODEL, max_tokens: 500, messages, tools });

    if (resp.stop_reason === "tool_use") {
      messages.push({ role: "assistant", content: resp.content });
      const results: Anthropic.Messages.ToolResultBlockParam[] = [];
      for (const tb of resp.content.filter((b): b is Anthropic.Messages.ToolUseBlock => b.type === "tool_use")) {
        const result = searchDocs((tb.input as { query: string }).query);
        console.log(`  [tool] ${tb.name}("${(tb.input as { query: string }).query}") → ${result}`);
        results.push({ type: "tool_result", tool_use_id: tb.id, content: result });
      }
      messages.push({ role: "user", content: results });
    } else {
      for (const b of resp.content) if (b.type === "text") console.log(`Assistant: ${b.text}`);
      break;
    }
  }

  // Streaming demo
  console.log("\n--- Streaming ---");
  process.stdout.write("Assistant: ");
  const stream = client.messages.stream({
    model: MODEL, max_tokens: 100,
    messages: [{ role: "user", content: "3 benefits of Go for building proxies?" }],
  });
  for await (const e of stream) if (e.type === "content_block_delta" && e.delta.type === "text_delta") process.stdout.write(e.delta.text);
  console.log();
}

main().catch(console.error);
