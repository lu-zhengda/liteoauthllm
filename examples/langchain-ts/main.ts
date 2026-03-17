/**
 * LangChain.js — ReAct agent with tools via Anthropic.
 *
 * Run: ANTHROPIC_BASE_URL=http://127.0.0.1:8639 npx tsx main.ts
 * Deps: npm install @langchain/anthropic @langchain/core zod
 */
import { ChatAnthropic } from "@langchain/anthropic";
import { HumanMessage, ToolMessage } from "@langchain/core/messages";
import { tool } from "@langchain/core/tools";
import { z } from "zod";

const MODEL = "claude-haiku-4-5-20251001";

const translateTool = tool(
  async ({ text, to }) => {
    const t: Record<string, Record<string, string>> = {
      ja: { hello: "こんにちは", "thank you": "ありがとう" },
      es: { hello: "hola", "thank you": "gracias" },
      fr: { hello: "bonjour", "thank you": "merci" },
    };
    return t[to.toLowerCase()]?.[text.toLowerCase()] ?? `No translation for "${text}" → ${to}`;
  },
  {
    name: "translate",
    description: "Translate a short phrase to another language",
    schema: z.object({ text: z.string(), to: z.string().describe("Language code: ja, es, fr") }),
  }
);

async function main() {
  const llm = new ChatAnthropic({
    model: MODEL, maxTokens: 500,
    anthropicApiUrl: process.env.ANTHROPIC_BASE_URL || "http://127.0.0.1:8639",
    anthropicApiKey: "proxy-injected",
  }).bindTools([translateTool]);

  const messages = [new HumanMessage("How do you say 'hello' and 'thank you' in Japanese and Spanish?")];
  console.log(`User: ${messages[0].content}`);

  while (true) {
    const resp = await llm.invoke(messages);
    messages.push(resp);

    if (resp.tool_calls?.length) {
      for (const tc of resp.tool_calls) {
        const result = await translateTool.invoke(tc.args);
        console.log(`  [tool] ${tc.name}(${JSON.stringify(tc.args)}) → ${result}`);
        messages.push(new ToolMessage({ content: String(result), tool_call_id: tc.id! }));
      }
    } else {
      console.log(`Assistant: ${resp.content}`);
      break;
    }
  }
}

main().catch(console.error);
