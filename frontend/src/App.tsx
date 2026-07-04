import { useEffect, useState } from "react";
import type { KeyboardEvent } from "react";
import type { ADKEvent, Agent, Message, RunResponse, RunStreamEvent, StudioApp } from "./types";

type SendShortcut = "enter" | "modified";
type EventContent = NonNullable<ADKEvent["Content"]>;
type ToolCall = NonNullable<EventContent["ToolCalls"]>[number];
type ToolResult = NonNullable<EventContent["ToolResult"]>;

export function App() {
  const [app, setApp] = useState<StudioApp | null>(null);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [selectedAgent, setSelectedAgent] = useState("");
  const [appID, setAppID] = useState("adk-studio");
  const [userID, setUserID] = useState("local-user");
  const [sessionID, setSessionID] = useState("session-1");
  const [input, setInput] = useState("");
  const [messages, setMessages] = useState<Message[]>([]);
  const [traceEvents, setTraceEvents] = useState<RunStreamEvent[]>([]);
  const [sendShortcut, setSendShortcut] = useState<SendShortcut>("enter");
  const [isRunning, setIsRunning] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    fetch("./api/app")
      .then((response) => response.json())
      .then((data: StudioApp) => setApp(data))
      .catch(() => setApp(null));

    fetch("./api/agents")
      .then((response) => response.json())
      .then((data: { agents: Agent[] }) => {
        setAgents(data.agents);
        setSelectedAgent((current) => current || data.agents[0]?.id || "");
      })
      .catch(() => setAgents([]));
  }, []);

  async function runAgent() {
    const prompt = input.trim();
    if (!prompt || !selectedAgent || isRunning) {
      return;
    }

    setError("");
    setIsRunning(true);
    setMessages((current) => [
      ...current,
      {
        id: `user-${Date.now()}`,
        role: "user",
        author: "user",
        content: prompt
      }
    ]);
    setInput("");

    try {
      const response = await fetch("./api/runs", {
        method: "POST",
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify({
          agent_id: selectedAgent,
          app_name: appID,
          user_id: userID,
          session_id: sessionID,
          input: {
            role: "user",
            content: prompt
          }
        })
      });

      const data = (await response.json()) as RunResponse | { error?: string; events?: RunStreamEvent[] };
      if (!response.ok) {
        const events = "events" in data && data.events ? data.events : [];
        const eventError = [...events].reverse().find((event) => event.error)?.error;
        const message = ("error" in data && data.error) || eventError || "Run failed";
        setError(message);
        setTraceEvents((current) => [...current, ...events]);
        setMessages((current) => [
          ...current,
          {
            id: `error-${Date.now()}`,
            role: "error",
            author: "error",
            content: message
          }
        ]);
        return;
      }

      const run = data as RunResponse;
      setSessionID(run.session_id);
      setTraceEvents((current) => [...current, ...run.events]);
      setMessages((current) => [...current, ...run.events.flatMap(eventToMessages)]);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Run failed";
      setError(message);
      setMessages((current) => [
        ...current,
        {
          id: `error-${Date.now()}`,
          role: "error",
          author: "error",
          content: message
        }
      ]);
    } finally {
      setIsRunning(false);
    }
  }

  function handleComposerKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key !== "Enter" || event.nativeEvent.isComposing) {
      return;
    }
    const hasModifier = event.shiftKey || event.metaKey || event.ctrlKey;
    const shouldSend = sendShortcut === "enter" ? !hasModifier && !event.altKey : hasModifier;
    if (!shouldSend) {
      return;
    }
    event.preventDefault();
    void runAgent();
  }

  return (
    <main className="studio-shell">
      <aside className="studio-sidebar" aria-label="Project controls">
        <div className="brand-block">
          <span className="brand-mark">ADK</span>
          <div>
            <h1>ADK Studio</h1>
            <p>{app?.name || "Embedded agent debugger"}</p>
          </div>
        </div>

        <section className="control-section">
          <h2>App</h2>
          <label>
            Name
            <input value={app?.name || "Loading"} readOnly />
          </label>
          <label>
            Agents
            <select value={selectedAgent} onChange={(event) => setSelectedAgent(event.target.value)}>
              {agents.length === 0 ? <option value="">No agents registered</option> : null}
              {agents.map((agent) => (
                <option key={agent.id} value={agent.id}>
                  {agent.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            Session Store
            <input value={app?.has_session_service ? "Configured" : "Not configured"} readOnly />
          </label>
        </section>

        <section className="control-section">
          <h2>Session</h2>
          <label>
            App ID
            <input value={appID} onChange={(event) => setAppID(event.target.value)} />
          </label>
          <label>
            User ID
            <input value={userID} onChange={(event) => setUserID(event.target.value)} />
          </label>
          <label>
            Session ID
            <input value={sessionID} onChange={(event) => setSessionID(event.target.value)} />
          </label>
        </section>
      </aside>

      <section className="playground" aria-label="Playground">
        <header className="workspace-header">
          <div>
            <h2>Playground</h2>
            <p>Runs will execute against the selected registered agent.</p>
          </div>
          <button type="button" onClick={runAgent} disabled={isRunning || !input.trim() || !selectedAgent}>
            {isRunning ? "Running" : "Run"}
          </button>
        </header>

        <div className="message-list">
          {messages.length === 0 ? (
            <article className="message assistant-message">
              <span>Studio</span>
              <p>Select an agent, type a prompt, then run it against the embedded ADK app.</p>
            </article>
          ) : null}
          {messages.map((message) => (
            <article key={message.id} className={`message ${message.role}-message`}>
              <span>{message.author}</span>
              {message.content ? <p>{message.content}</p> : null}
              {message.reasoning ? (
                <details className="reasoning-block" open>
                  <summary>Reasoning</summary>
                  <pre>{message.reasoning}</pre>
                </details>
              ) : null}
            </article>
          ))}
        </div>

        {error ? <div className="error-banner">{error}</div> : null}

        <form
          className="composer"
          onSubmit={(event) => {
            event.preventDefault();
            runAgent();
          }}
        >
          <div className="composer-input-row">
            <textarea
              placeholder="Type a prompt for the ADK runner"
              rows={2}
              value={input}
              onChange={(event) => setInput(event.target.value)}
              onKeyDown={handleComposerKeyDown}
            />
            <button type="submit" disabled={isRunning || !input.trim() || !selectedAgent}>
              {isRunning ? "Sending" : "Send"}
            </button>
          </div>
          <div className="composer-toolbar">
            <div className="segmented-control" role="group" aria-label="Send shortcut">
              <button
                type="button"
                className={sendShortcut === "enter" ? "is-active" : undefined}
                aria-pressed={sendShortcut === "enter"}
                onClick={() => setSendShortcut("enter")}
              >
                Enter
              </button>
              <button
                type="button"
                className={sendShortcut === "modified" ? "is-active" : undefined}
                aria-pressed={sendShortcut === "modified"}
                onClick={() => setSendShortcut("modified")}
              >
                Shift / Cmd Enter
              </button>
            </div>
          </div>
        </form>
      </section>

      <aside className="inspector" aria-label="Event inspector">
        <header>
          <h2>Trace</h2>
          <p>Agent events, tool calls, usage, and errors.</p>
        </header>
        {traceEvents.length === 0 ? (
          <div className="empty-state">
            <strong>No run selected</strong>
            <span>Run events will be listed chronologically.</span>
          </div>
        ) : (
          <div className="trace-list">
            {traceEvents.map((trace, index) => (
              <details key={`${trace.run_id}-${index}`} className="trace-item">
                <summary>
                  <span>{trace.type}</span>
                  <strong>{trace.event?.Author || trace.error || "event"}</strong>
                </summary>
                <pre>{JSON.stringify(trace, null, 2)}</pre>
              </details>
            ))}
          </div>
        )}
      </aside>
    </main>
  );
}

function eventToMessages(trace: RunStreamEvent): Message[] {
  if (trace.type === "error") {
    return [
      {
        id: `${trace.run_id}-error`,
        role: "error",
        author: "error",
        content: trace.error || "Run failed"
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
      content: formatToolResult(toolResult)
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
        content: formatToolCall(call)
      }))
    );
  }

  if (content.ToolResult) {
    messages.push({
      id: `${idPrefix}-tool-result-${content.ToolResult.ToolCallID || "result"}`,
      role: "tool_result",
      author: toolResultAuthor(content.ToolResult),
      content: formatToolResult(content.ToolResult)
    });
  }

  return messages;
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

function toolCallAuthor(call: ToolCall): string {
  return `tool call: ${call.Name || call.ID || "tool"}`;
}

function toolResultAuthor(result: Partial<ToolResult>): string {
  return `tool result: ${result?.Name || result?.ToolCallID || "tool"}`;
}

function formatToolCall(call: ToolCall): string {
  const lines = [`name: ${call.Name || "tool"}`];
  if (call.ID) {
    lines.push(`id: ${call.ID}`);
  }
  if (call.Arguments !== undefined && call.Arguments !== null) {
    lines.push(`arguments:\n${formatValue(call.Arguments)}`);
  }
  return lines.join("\n");
}

function formatToolResult(result: Partial<ToolResult>): string {
  const lines = [`status: ${result?.IsError ? "error" : "ok"}`];
  if (result?.Name) {
    lines.push(`name: ${result.Name}`);
  }
  if (result?.ToolCallID) {
    lines.push(`id: ${result.ToolCallID}`);
  }
  if (result?.Content) {
    lines.push(`content:\n${result.Content}`);
  }
  if (result?.StructuredContent !== undefined && result.StructuredContent !== null) {
    lines.push(`structured:\n${formatValue(result.StructuredContent)}`);
  }
  return lines.join("\n");
}

function formatValue(value: unknown): string {
  if (typeof value === "string") {
    try {
      return JSON.stringify(JSON.parse(value), null, 2);
    } catch {
      return value;
    }
  }

  const json = JSON.stringify(value, null, 2);
  return json || String(value);
}
