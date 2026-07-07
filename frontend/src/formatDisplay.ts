import type { RunStreamEvent } from "./types";

export type FormattedBlock = {
  value: string;
  language: "json" | "text";
};

export function formatBlock(value: unknown): string {
  const block = formatValue(value);
  return fencedCode(block.value, block.language);
}

export function formatTraceEvent(trace: RunStreamEvent): string {
  return JSON.stringify(formatTraceValue(trace), null, 2);
}

function formatValue(value: unknown): FormattedBlock {
  if (typeof value === "string") {
    try {
      return {
        value: JSON.stringify(JSON.parse(value), null, 2),
        language: "json"
      };
    } catch {
      return {
        value,
        language: "text"
      };
    }
  }

  try {
    const json = JSON.stringify(value, null, 2);
    if (json) {
      return {
        value: json,
        language: "json"
      };
    }
  } catch {
    // Fall through to a plain text representation for non-serializable values.
  }

  return {
    value: String(value),
    language: "text"
  };
}

function formatTraceValue(value: unknown): unknown {
  if (typeof value === "string") {
    return formatTraceString(value);
  }
  if (Array.isArray(value)) {
    return value.map(formatTraceValue);
  }
  if (isRecord(value)) {
    return Object.fromEntries(Object.entries(value).map(([key, entry]) => [key, formatTraceValue(entry)]));
  }
  return value;
}

function formatTraceString(value: string): unknown {
  const trimmed = value.trim();
  if (!trimmed || (!trimmed.startsWith("{") && !trimmed.startsWith("["))) {
    return value;
  }

  try {
    return formatTraceValue(JSON.parse(trimmed));
  } catch {
    return value;
  }
}

function fencedCode(value: string, language: FormattedBlock["language"]): string {
  const fence = longestBacktickRun(value) >= 3 ? "`".repeat(longestBacktickRun(value) + 1) : "```";
  return `${fence}${language}\n${value}\n${fence}`;
}

function longestBacktickRun(value: string): number {
  return Math.max(0, ...Array.from(value.matchAll(/`+/g), (match) => match[0].length));
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}
