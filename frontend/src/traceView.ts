import type { RunStreamEvent } from "./types";

export function traceTypeLabel(trace: RunStreamEvent): string {
  if (trace.type === "error") {
    return "error";
  }
  const content = trace.event?.Content;
  if (content?.Role === "tool") {
    return "tool result";
  }
  if (content?.ToolResult) {
    return "tool result";
  }
  if (content?.ToolCalls?.length) {
    return "tool call";
  }
  return content?.Role || trace.type;
}

export function traceTitle(trace: RunStreamEvent): string {
  return trace.event?.Author || trace.error || "event";
}

export function traceTimeLabel(trace: RunStreamEvent): string {
  const date = traceTime(trace);
  if (!date) {
    return "--:--:--";
  }
  return date.toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit"
  });
}

export function traceTimeISO(trace: RunStreamEvent): string {
  return traceTime(trace)?.toISOString() || "";
}

function traceTime(trace: RunStreamEvent): Date | null {
  const timestamp = trace.event?.CreatedAt || trace.event?.UpdatedAt || trace.received_at;
  if (!timestamp) {
    return null;
  }
  return new Date(timestamp);
}
