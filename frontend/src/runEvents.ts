import type { ADKEvent, Message, RunStreamEvent } from "./types";
import { formatBlock } from "./formatDisplay";

type EventContent = NonNullable<ADKEvent["Content"]>;
type ToolCall = NonNullable<EventContent["ToolCalls"]>[number];
type ToolResult = NonNullable<EventContent["ToolResult"]>;

export function completeRunEvents(events: RunStreamEvent[]): RunStreamEvent[] {
  return events.filter((event) => event.type !== "partial" && !event.event?.Partial);
}

export function markRunEventReceived(event: RunStreamEvent): RunStreamEvent {
  return {
    ...event,
    received_at: event.received_at || Date.now()
  };
}

export function isTraceVisible(trace: RunStreamEvent): boolean {
  return trace.type !== "partial" && !trace.event?.Partial;
}

export async function readRunEventStream(response: Response, onEvent: (event: RunStreamEvent) => void) {
  if (!response.body) {
    throw new Error("Streaming response body is unavailable");
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  while (true) {
    const { value, done } = await reader.read();
    if (value) {
      buffer += decoder.decode(value, { stream: true });
      buffer = consumeRunStreamFrames(buffer, onEvent);
    }
    if (done) {
      buffer += decoder.decode();
      break;
    }
  }

  const tail = buffer.trim();
  if (tail) {
    const event = parseRunStreamFrame(tail);
    if (event) {
      onEvent(event);
    }
  }
}

export function applyRunStreamEvent(current: Message[], trace: RunStreamEvent): Message[] {
  if (trace.type === "error") {
    return [...current, ...eventToMessages(trace)];
  }
  if (!trace.event) {
    return current;
  }
  if (trace.type === "partial" || trace.event.Partial) {
    return upsertPartialMessage(current, trace);
  }

  const messages = eventToMessages(trace);
  const partialID = partialMessageID(trace);
  const partialIndex = current.findIndex((message) => message.id === partialID);
  if (partialIndex < 0) {
    return messages.length > 0 ? [...current, ...messages] : current;
  }
  if (messages.length === 0) {
    return current.filter((_, index) => index !== partialIndex);
  }
  return [...current.slice(0, partialIndex), ...messages, ...current.slice(partialIndex + 1)];
}

export function eventToMessages(trace: RunStreamEvent): Message[] {
  if (trace.type === "partial" || trace.event?.Partial) {
    return [];
  }
  if (trace.type === "error") {
    return [
      {
        id: `${trace.run_id}-error`,
        role: "error",
        author: "error",
        content: trace.error || "Run failed",
        createdAt: messageTimestamp(trace)
      }
    ];
  }
  if (!trace.event) {
    return [];
  }
  const content = trace.event.Content;
  if (!content) {
    return [];
  }

  const idPrefix = `${trace.run_id}-${trace.event.ID || Date.now()}-${trace.event.Author || "event"}`;
  const messages: Message[] = [];
  if (content.Role === "tool") {
    const toolResult = content.ToolResult || {
      ToolCallID: content.ToolCallID,
      Content: content.Content
    };
    messages.push({
      id: `${idPrefix}-tool-result`,
      role: "tool_result",
      author: toolResultAuthor(toolResult),
      content: formatToolResult(toolResult),
      createdAt: messageTimestamp(trace)
    });
    return messages;
  }

  const text = content.Content || "";
  const reasoning = eventReasoning(trace.event);
  const role = eventRole(trace.event);
  if (text || reasoning) {
    messages.push({
      id: `${idPrefix}-${role}-${text.length}-${reasoning.length}`,
      role,
      author: trace.event.Author || role,
      content: text,
      createdAt: messageTimestamp(trace),
      reasoning,
      partial: trace.event.Partial
    });
  }

  if (content.ToolCalls?.length) {
    messages.push(
      ...content.ToolCalls.map((call, index) => ({
        id: `${idPrefix}-tool-call-${call.ID || index}`,
        role: "tool_call" as const,
        author: toolCallAuthor(call),
        content: formatToolCall(call),
        createdAt: messageTimestamp(trace)
      }))
    );
  }

  if (content.ToolResult) {
    messages.push({
      id: `${idPrefix}-tool-result-${content.ToolResult.ToolCallID || "result"}`,
      role: "tool_result",
      author: toolResultAuthor(content.ToolResult),
      content: formatToolResult(content.ToolResult),
      createdAt: messageTimestamp(trace)
    });
  }

  return messages;
}

function consumeRunStreamFrames(buffer: string, onEvent: (event: RunStreamEvent) => void): string {
  let next = buffer;
  let boundary = next.indexOf("\n\n");
  while (boundary >= 0) {
    const frame = next.slice(0, boundary);
    next = next.slice(boundary + 2);
    const event = parseRunStreamFrame(frame);
    if (event) {
      onEvent(event);
    }
    boundary = next.indexOf("\n\n");
  }
  return next;
}

function parseRunStreamFrame(frame: string): RunStreamEvent | null {
  const data = frame
    .split(/\r?\n/)
    .filter((line) => line.startsWith("data:"))
    .map((line) => line.slice("data:".length).trimStart())
    .join("\n");
  if (!data) {
    return null;
  }
  return JSON.parse(data) as RunStreamEvent;
}

function upsertPartialMessage(current: Message[], trace: RunStreamEvent): Message[] {
  if (!trace.event?.Content) {
    return current;
  }

  const text = trace.event.Content.Content || "";
  const reasoning = eventReasoning(trace.event);
  if (!text && !reasoning) {
    return current;
  }

  const id = partialMessageID(trace);
  const index = current.findIndex((message) => message.id === id);
  if (index < 0) {
    const role = eventRole(trace.event);
    return [
      ...current,
      {
        id,
        role,
        author: trace.event.Author || role,
        content: text,
        createdAt: messageTimestamp(trace),
        reasoning: reasoning || undefined,
        partial: true
      }
    ];
  }

  const existing = current[index];
  const nextReasoning = `${existing.reasoning || ""}${reasoning}`;
  const updated: Message = {
    ...existing,
    content: `${existing.content}${text}`,
    createdAt: existing.createdAt || messageTimestamp(trace),
    reasoning: nextReasoning || undefined,
    partial: true
  };
  return [...current.slice(0, index), updated, ...current.slice(index + 1)];
}

function partialMessageID(trace: RunStreamEvent): string {
  const event = trace.event;
  return `${trace.run_id}-partial-${event?.Author || event?.Content?.Role || "event"}`;
}

function eventRole(event: ADKEvent): Message["role"] {
  const role = event.Content?.Role;
  if (role === "system") {
    return "system";
  }
  return "assistant";
}

function eventReasoning(event: ADKEvent): string {
  return event.Content?.ReasoningContent || "";
}

function messageTimestamp(trace: RunStreamEvent): number | undefined {
  return trace.event?.CreatedAt ?? trace.event?.UpdatedAt ?? trace.received_at;
}

function toolCallAuthor(call: ToolCall): string {
  return `tool call: ${call.Name || call.ID || "tool"}`;
}

function toolResultAuthor(result: Partial<ToolResult>): string {
  return `tool result: ${result?.Name || result?.ToolCallID || "tool"}`;
}

function formatToolCall(call: ToolCall): string {
  const lines = [`**name:** ${call.Name || "tool"}`];
  if (call.ID) {
    lines.push(`**id:** ${call.ID}`);
  }
  if (call.Arguments !== undefined && call.Arguments !== null) {
    lines.push("**arguments:**", formatBlock(call.Arguments));
  }
  return lines.join("\n\n");
}

function formatToolResult(result: Partial<ToolResult>): string {
  const lines = [`**status:** ${result?.IsError ? "error" : "ok"}`];
  if (result?.Name) {
    lines.push(`**name:** ${result.Name}`);
  }
  if (result?.ToolCallID) {
    lines.push(`**id:** ${result.ToolCallID}`);
  }
  const payload = toolResultPayload(result);
  if (payload !== undefined && payload !== null && payload !== "") {
    lines.push("**result:**", formatBlock(payload));
  }
  return lines.join("\n\n");
}

function toolResultPayload(result: Partial<ToolResult>): unknown {
  if (result?.StructuredContent !== undefined && result.StructuredContent !== null) {
    return result.StructuredContent;
  }
  return result?.Content;
}
