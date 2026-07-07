import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { KeyboardEvent } from "react";
import { CreateSessionDialog } from "./components/CreateSessionDialog";
import { Playground } from "./components/Playground";
import { Sidebar } from "./components/Sidebar";
import { TraceInspector } from "./components/TraceInspector";
import {
  applyRunStreamEvent,
  completeRunEvents,
  eventToMessages,
  isTraceVisible,
  markRunEventReceived,
  readRunEventStream
} from "./runEvents";
import type { Agent, Message, RunResponse, RunStreamEvent, StudioApp } from "./types";
import type { SendShortcut, SessionDraft, StudioSession } from "./uiTypes";

const initialSessionKey = "initial-session";

export function App() {
  const [app, setApp] = useState<StudioApp | null>(null);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [selectedAgent, setSelectedAgent] = useState("");
  const [appID, setAppID] = useState("adk-studio");
  const [userID, setUserID] = useState("local-user");
  const [sessions, setSessions] = useState<StudioSession[]>(() => [
    createStudioSession({ id: newSessionID(), title: "New session" }, initialSessionKey)
  ]);
  const [activeSessionKey, setActiveSessionKey] = useState(initialSessionKey);
  const [sessionDraft, setSessionDraft] = useState<SessionDraft>(() => newSessionDraft());
  const [isCreateSessionOpen, setIsCreateSessionOpen] = useState(false);
  const [input, setInput] = useState("");
  const [sendShortcut, setSendShortcut] = useState<SendShortcut>("enter");
  const [streamingEnabled, setStreamingEnabled] = useState(true);
  const [isRunning, setIsRunning] = useState(false);
  const [error, setError] = useState("");
  const messageListRef = useRef<HTMLDivElement | null>(null);
  const activeSession = sessions.find((session) => session.key === activeSessionKey) || sessions[0];
  const sessionID = activeSession?.id || "";
  const messages = activeSession?.messages || [];
  const traceEvents = activeSession?.traceEvents || [];
  const sessionDraftID = sessionDraft.id.trim();
  const isSessionDraftDuplicate = sessions.some((session) => session.id === sessionDraftID);

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

  useLayoutEffect(() => {
    const list = messageListRef.current;
    if (!list) {
      return;
    }
    list.scrollTop = list.scrollHeight;
  }, [messages]);

  function openCreateSessionDialog() {
    setSessionDraft(newSessionDraft(sessions.length + 1));
    setIsCreateSessionOpen(true);
    setError("");
  }

  function closeCreateSessionDialog() {
    setIsCreateSessionOpen(false);
  }

  function createSession() {
    const id = sessionDraftID;
    if (!id || isSessionDraftDuplicate) {
      return;
    }
    const session = createStudioSession({
      id,
      title: sessionDraft.title.trim() || "Untitled session"
    });
    setSessions((current) => [...current, session]);
    setActiveSessionKey(session.key);
    setIsCreateSessionOpen(false);
    setError("");
  }

  function selectSession(sessionKey: string) {
    setActiveSessionKey(sessionKey);
    setError("");
  }

  function updateActiveSessionID(id: string) {
    if (!activeSession) {
      return;
    }
    updateSession(activeSession.key, (session) => ({
      ...session,
      id
    }));
  }

  function syncSessionID(sessionKey: string, id?: string) {
    if (!id) {
      return;
    }
    updateSession(sessionKey, (session) => ({
      ...session,
      id
    }));
  }

  function updateSession(sessionKey: string, updater: (session: StudioSession) => StudioSession) {
    setSessions((current) => current.map((session) => (session.key === sessionKey ? updater(session) : session)));
  }

  function updateSessionMessages(sessionKey: string, updater: (messages: Message[]) => Message[]) {
    updateSession(sessionKey, (session) => ({
      ...session,
      messages: updater(session.messages)
    }));
  }

  function updateSessionTraceEvents(sessionKey: string, updater: (events: RunStreamEvent[]) => RunStreamEvent[]) {
    updateSession(sessionKey, (session) => ({
      ...session,
      traceEvents: updater(session.traceEvents)
    }));
  }

  async function runAgent() {
    const prompt = input.trim();
    const runSession = activeSession;
    const runSessionID = runSession?.id.trim();
    if (!prompt || !selectedAgent || isRunning || !runSession || !runSessionID) {
      return;
    }

    setError("");
    setIsRunning(true);
    const sentAt = Date.now();
    updateSessionMessages(runSession.key, (current) => [
      ...current,
      {
        id: `user-${sentAt}`,
        role: "user",
        author: "user",
        content: prompt,
        createdAt: sentAt
      }
    ]);
    setInput("");

    try {
      const headers: Record<string, string> = {
        "Content-Type": "application/json"
      };
      if (streamingEnabled) {
        headers.Accept = "text/event-stream";
      }

      const response = await fetch("./api/runs", {
        method: "POST",
        headers,
        body: JSON.stringify({
          agent_id: selectedAgent,
          app_name: appID,
          user_id: userID,
          session_id: runSessionID,
          input: {
            role: "user",
            content: prompt
          }
        })
      });

      if (streamingEnabled && response.ok && response.headers.get("Content-Type")?.includes("text/event-stream")) {
        await readRunEventStream(response, (event) => {
          if (event.session_id) {
            syncSessionID(runSession.key, event.session_id);
          }
          if (event.type === "done") {
            return;
          }
          if (event.type === "error") {
            setError(event.error || "Run failed");
          }
          const receivedEvent = markRunEventReceived(event);
          if (isTraceVisible(receivedEvent)) {
            updateSessionTraceEvents(runSession.key, (current) => [...current, receivedEvent]);
          }
          updateSessionMessages(runSession.key, (current) => applyRunStreamEvent(current, receivedEvent));
        });
        return;
      }

      const data = (await response.json()) as RunResponse | { error?: string; events?: RunStreamEvent[] };
      if (!response.ok) {
        const events = "events" in data && data.events ? completeRunEvents(data.events) : [];
        const eventError = [...events].reverse().find((event) => event.error)?.error;
        const message = ("error" in data && data.error) || eventError || "Run failed";
        const failedAt = Date.now();
        setError(message);
        updateSessionTraceEvents(runSession.key, (current) => [
          ...current,
          ...events.map(markRunEventReceived).filter(isTraceVisible)
        ]);
        updateSessionMessages(runSession.key, (current) => [
          ...current,
          {
            id: `error-${failedAt}`,
            role: "error",
            author: "error",
            content: message,
            createdAt: failedAt
          }
        ]);
        return;
      }

      const run = data as RunResponse;
      const events = completeRunEvents(run.events).map(markRunEventReceived);
      syncSessionID(runSession.key, run.session_id);
      updateSessionTraceEvents(runSession.key, (current) => [
        ...current,
        ...events.filter(isTraceVisible)
      ]);
      updateSessionMessages(runSession.key, (current) => [...current, ...events.flatMap(eventToMessages)]);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Run failed";
      const failedAt = Date.now();
      setError(message);
      updateSessionMessages(runSession.key, (current) => [
        ...current,
        {
          id: `error-${failedAt}`,
          role: "error",
          author: "error",
          content: message,
          createdAt: failedAt
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
    <main className="grid h-dvh min-h-0 grid-cols-1 grid-rows-[auto_minmax(0,1fr)] overflow-hidden bg-background text-foreground md:grid-cols-[260px_minmax(0,1fr)] md:grid-rows-1 xl:grid-cols-[300px_minmax(0,1fr)_340px]">
      <Sidebar
        app={app}
        agents={agents}
        selectedAgent={selectedAgent}
        appID={appID}
        userID={userID}
        sessions={sessions}
        activeSessionKey={activeSessionKey}
        sessionID={sessionID}
        onSelectedAgentChange={setSelectedAgent}
        onAppIDChange={setAppID}
        onUserIDChange={setUserID}
        onOpenCreateSession={openCreateSessionDialog}
        onSelectSession={selectSession}
        onActiveSessionIDChange={updateActiveSessionID}
      />

      {isCreateSessionOpen ? (
        <CreateSessionDialog
          draft={sessionDraft}
          draftID={sessionDraftID}
          isDuplicate={isSessionDraftDuplicate}
          onDraftChange={setSessionDraft}
          onCreate={createSession}
          onClose={closeCreateSessionDialog}
        />
      ) : null}

      <Playground
        messages={messages}
        error={error}
        input={input}
        isRunning={isRunning}
        selectedAgent={selectedAgent}
        sessionID={sessionID}
        sendShortcut={sendShortcut}
        streamingEnabled={streamingEnabled}
        messageListRef={messageListRef}
        onInputChange={setInput}
        onRun={runAgent}
        onSendShortcutChange={setSendShortcut}
        onStreamingEnabledChange={setStreamingEnabled}
        onComposerKeyDown={handleComposerKeyDown}
      />

      <TraceInspector events={traceEvents.filter(isTraceVisible)} />
    </main>
  );
}

function createStudioSession(
  draft: SessionDraft,
  key = `session-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
): StudioSession {
  return {
    key,
    id: draft.id,
    title: draft.title,
    messages: [],
    traceEvents: []
  };
}

function newSessionDraft(index = 1): SessionDraft {
  return {
    id: newSessionID(),
    title: `Session ${index}`
  };
}

function newSessionID(): string {
  return `session-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
}
