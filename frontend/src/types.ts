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
  TurnID?: string;
  Author?: string;
  Content?: ADKContent;
  FinishReason?: string;
  Usage?: {
    PromptTokens?: number;
    CompletionTokens?: number;
    TotalTokens?: number;
    Details?: {
      cached_prompt_tokens?: number;
      cache_creation_prompt_tokens?: number;
      cache_read_prompt_tokens?: number;
      reasoning_tokens?: number;
      tool_use_prompt_tokens?: number;
      audio_prompt_tokens?: number;
      audio_completion_tokens?: number;
      accepted_prediction_tokens?: number;
      rejected_prediction_tokens?: number;
    };
  };
  Partial?: boolean;
  CreatedAt?: number;
  UpdatedAt?: number;
};

export type RunTraceRecord = {
  phase: "start" | "event" | "end" | string;
  kind: string;
  time: string;
  duration?: number;
  runtime_run_id?: string;
  turn_id?: string;
  session_id?: string;
  app_id?: string;
  user_id?: string;
  agent_name?: string;
  model?: string;
  iteration?: number;
  stream?: boolean;
  event_id?: string;
  event_author?: string;
  event_role?: string;
  event_count?: number;
  partial?: boolean;
  tool_name?: string;
  tool_call_id?: string;
  tool_index?: number;
  finish_reason?: string;
  prompt_tokens?: number;
  completion_tokens?: number;
  total_tokens?: number;
  partial_responses?: number;
  stopped_early?: boolean;
  is_error?: boolean;
  error?: string;
  attributes?: Record<string, unknown>;
};

export type RunFailure = {
  code: "run_failed" | "tool_execution_unknown" | string;
  message: string;
  session_id?: string;
  source_turn_id?: string;
  source_event_id?: string;
  unresolved_tools?: Array<{ id?: string; name?: string }>;
};

export type RunStreamEvent = {
  type: "event" | "partial" | "trace" | "error" | "done" | string;
  run_id: string;
  session_id?: string;
  event?: ADKEvent;
  trace?: RunTraceRecord;
  failure?: RunFailure;
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
  runId?: string;
  failed?: boolean;
  persisted?: boolean;
};
