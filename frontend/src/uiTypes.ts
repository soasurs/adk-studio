import type { Message, RunStreamEvent } from "./types";

export type SendShortcut = "enter" | "modified";

export type StudioSession = {
  key: string;
  id: string;
  title: string;
  messages: Message[];
  traceEvents: RunStreamEvent[];
};

export type SessionDraft = {
  id: string;
  title: string;
};
