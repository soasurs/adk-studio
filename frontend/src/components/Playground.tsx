import type { KeyboardEvent, RefObject } from "react";
import type { Message } from "../types";
import type { SendShortcut } from "../uiTypes";
import { MessageItem } from "./MessageItem";

type PlaygroundProps = {
  messages: Message[];
  error: string;
  input: string;
  isRunning: boolean;
  selectedAgent: string;
  sessionID: string;
  sendShortcut: SendShortcut;
  streamingEnabled: boolean;
  messageListRef: RefObject<HTMLDivElement | null>;
  onInputChange: (value: string) => void;
  onRun: () => void;
  onSendShortcutChange: (shortcut: SendShortcut) => void;
  onStreamingEnabledChange: (enabled: boolean) => void;
  onComposerKeyDown: (event: KeyboardEvent<HTMLTextAreaElement>) => void;
};

export function Playground({
  messages,
  error,
  input,
  isRunning,
  selectedAgent,
  sessionID,
  sendShortcut,
  streamingEnabled,
  messageListRef,
  onInputChange,
  onRun,
  onSendShortcutChange,
  onStreamingEnabledChange,
  onComposerKeyDown
}: PlaygroundProps) {
  const runDisabled = isRunning || !input.trim() || !selectedAgent || !sessionID.trim();

  return (
    <section className="playground" aria-label="Playground">
      <header className="workspace-header">
        <div>
          <h2>Playground</h2>
          <p>Runs will execute against the selected registered agent.</p>
        </div>
        <button type="button" onClick={onRun} disabled={runDisabled}>
          {isRunning ? "Running" : "Run"}
        </button>
      </header>

      <div className="message-list" ref={messageListRef}>
        {messages.length === 0 ? (
          <article className="message assistant-message">
            <span>Studio</span>
            <p>Select an agent, type a prompt, then run it against the embedded ADK app.</p>
          </article>
        ) : null}
        {messages.map((message) => (
          <MessageItem key={message.id} message={message} />
        ))}
      </div>

      {error ? <div className="error-banner">{error}</div> : null}

      <form
        className="composer"
        onSubmit={(event) => {
          event.preventDefault();
          onRun();
        }}
      >
        <div className="composer-input-row">
          <textarea
            placeholder="Type a prompt for the ADK runner"
            rows={2}
            value={input}
            onChange={(event) => onInputChange(event.target.value)}
            onKeyDown={onComposerKeyDown}
          />
          <button type="submit" disabled={runDisabled}>
            {isRunning ? "Sending" : "Send"}
          </button>
        </div>
        <div className="composer-toolbar">
          <label className="stream-toggle">
            <input
              type="checkbox"
              checked={streamingEnabled}
              disabled={isRunning}
              onChange={(event) => onStreamingEnabledChange(event.target.checked)}
            />
            Streaming
          </label>
          <div className="segmented-control" role="group" aria-label="Send shortcut">
            <button
              type="button"
              className={sendShortcut === "enter" ? "is-active" : undefined}
              aria-pressed={sendShortcut === "enter"}
              onClick={() => onSendShortcutChange("enter")}
            >
              Enter
            </button>
            <button
              type="button"
              className={sendShortcut === "modified" ? "is-active" : undefined}
              aria-pressed={sendShortcut === "modified"}
              onClick={() => onSendShortcutChange("modified")}
            >
              Shift / Cmd Enter
            </button>
          </div>
        </div>
      </form>
    </section>
  );
}
