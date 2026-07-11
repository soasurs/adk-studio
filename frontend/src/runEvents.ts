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
  return trace.type !== "partial" && trace.type !== "done" && !trace.event?.Partial;
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

export function applyRunStreamEvent(current: Message[], trace: RunStreamEvent, userMessageID?: string): Message[] {
  const associated = associateUserMessage(current, trace.run_id, userMessageID);
  if (trace.type === "done") {
    return associated.map((message) =>
      message.id === userMessageID ? { ...message, failed: false, persisted: true } : message
    );
  }
  if (trace.type === "error") {
    const cleaned = associated.filter(
      (message) =>
        message.runId !== trace.run_id ||
        !["assistant", "tool_call", "tool_result"].includes(message.role)
    );
    const failed = cleaned.map((message) =>
      message.id === userMessageID ? { ...message, failed: true, persisted: false } : message
    );
    return [...failed, ...eventToMessages(trace)];
  }
  if (!trace.event) {
    return associated;
  }
  if (trace.type === "partial" || trace.event.Partial) {
    return upsertPartialMessage(associated, trace);
  }

  const messages = eventToMessages(trace);
  const partialID = partialMessageID(trace);
  const partialIndex = associated.findIndex((message) => message.id === partialID);
  if (partialIndex < 0) {
    return messages.length > 0 ? [...associated, ...messages] : associated;
  }
  if (messages.length === 0) {
    return associated.filter((_, index) => index !== partialIndex);
  }
  return [...associated.slice(0, partialIndex), ...messages, ...associated.slice(partialIndex + 1)];
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
        content: runFailureMessage(trace),
        createdAt: messageTimestamp(trace),
        runId: trace.run_id
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
      createdAt: messageTimestamp(trace),
      runId: trace.run_id
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
      partial: trace.event.Partial,
      runId: trace.run_id
    });
  }

  if (content.ToolCalls?.length) {
    messages.push(
      ...content.ToolCalls.map((call, index) => ({
        id: `${idPrefix}-tool-call-${call.ID || index}`,
        role: "tool_call" as const,
        author: toolCallAuthor(call),
        content: formatToolCall(call),
        createdAt: messageTimestamp(trace),
        runId: trace.run_id
      }))
    );
  }

  if (content.ToolResult) {
    messages.push({
      id: `${idPrefix}-tool-result-${content.ToolResult.ToolCallID || "result"}`,
      role: "tool_result",
      author: toolResultAuthor(content.ToolResult),
      content: formatToolResult(content.ToolResult),
      createdAt: messageTimestamp(trace),
      runId: trace.run_id
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
        partial: true,
        runId: trace.run_id
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

function associateUserMessage(current: Message[], runID: string, userMessageID?: string): Message[] {
  if (!userMessageID) {
    return current;
  }
  return current.map((message) =>
    message.id === userMessageID && message.runId !== runID ? { ...message, runId: runID } : message
  );
}

export function runFailureMessage(trace: RunStreamEvent): string {
  if (trace.failure?.code !== "tool_execution_unknown") {
    return trace.failure?.message || trace.error || "Run failed";
  }
  const tools = trace.failure.unresolved_tools
    ?.map((call) => call.name || call.id)
    .filter(Boolean)
    .join(", ");
  const suffix = tools ? ` Unresolved tools: ${tools}.` : "";
  return `Tool execution status is unknown.${suffix} Studio will not retry these calls automatically. Start a new session or repair the persisted history before running this session again.`;
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
