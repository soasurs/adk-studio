export type Agent = {
  id: string;
  name: string;
  description?: string;
};

export type SessionBackend = {
  id: string;
  name: string;
  description?: string;
};

export type StudioApp = {
  name: string;
  agent_count: number;
  has_session_service: boolean;
  session_backend?: SessionBackend;
};

export type ADKContent = {
  Role?: string;
  Content?: string;
  ReasoningContent?: string;
  ToolCallID?: string;
  ToolCalls?: Array<{
    ID?: string;
    Name?: string;
    Arguments?: unknown;
  }>;
  ToolResult?: {
    ToolCallID?: string;
    Name?: string;
    Content?: string;
    StructuredContent?: unknown;
    IsError?: boolean;
  };
};

export type ADKEvent = {
  ID?: number;
  SessionID?: string;
  Author?: string;
  Content?: ADKContent;
  FinishReason?: string;
  Usage?: {
    PromptTokens?: number;
    CompletionTokens?: number;
    TotalTokens?: number;
  };
  Partial?: boolean;
  CreatedAt?: number;
  UpdatedAt?: number;
};

export type RunStreamEvent = {
  type: "event" | "partial" | "error" | "done" | string;
  run_id: string;
  session_id?: string;
  event?: ADKEvent;
  error?: string;
  received_at?: number;
};

export type RunResponse = {
  run_id: string;
  session_id: string;
  events: RunStreamEvent[];
  error?: string;
};

export type Message = {
  id: string;
  role: "user" | "assistant" | "tool_call" | "tool_result" | "system" | "error";
  author: string;
  content: string;
  createdAt?: number;
  reasoning?: string;
  partial?: boolean;
};
