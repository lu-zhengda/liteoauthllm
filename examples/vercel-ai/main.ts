/**
 * Vercel AI SDK — Multi-step agent with generateText and streamText.
 *
 * Run: ANTHROPIC_BASE_URL=http://127.0.0.1:8639 npx tsx main.ts
 * Deps: npm install ai @ai-sdk/anthropic zod
 */
import { generateText, streamText, tool } from "ai";
import { createAnthropic } from "@ai-sdk/anthropic";
import { z } from "zod";

// Vercel AI SDK's Anthropic adapter appends /messages directly to baseURL
// (unlike the official SDK which prepends /v1/), so we include /v1 in the baseURL
const anthropic = createAnthropic({
  baseURL: (process.env.ANTHROPIC_BASE_URL || "http://127.0.0.1:8639") + "/v1",
  apiKey: "proxy-injected",
});
const MODEL = anthropic("claude-haiku-4-5-20251001");

async function main() {
  // generateText with tools
  console.log("--- generateText with tools ---");
  const result = await generateText({
    model: MODEL, maxTokens: 500, maxSteps: 3,
    tools: {
      lookup_recipe: tool({
        description: "Look up a recipe by dish name",
        parameters: z.object({ dish: z.string() }),
        execute: async ({ dish }) => {
          console.log(`  [tool] lookup_recipe("${dish}")`);
          const recipes: Record<string, string> = {
            "pasta carbonara": "Spaghetti, eggs, pecorino, guanciale, pepper.",
            "miso soup": "Dashi, miso, tofu, wakame, green onion.",
          };
          return recipes[dish.toLowerCase()] ?? "Not found";
        },
      }),
    },
    prompt: "How do I make pasta carbonara? Look up the recipe.",
  });
  console.log(`Assistant: ${result.text}`);
  console.log(`  Steps: ${result.steps.length}`);

  // streamText
  console.log("\n--- streamText ---");
  process.stdout.write("Assistant: ");
  const stream = streamText({ model: MODEL, maxTokens: 100, prompt: "3 reasons to use a local AI proxy?" });
  for await (const chunk of stream.textStream) process.stdout.write(chunk);
  console.log();
}

main().catch(console.error);
