import type { Agent, StudioApp } from "../types";
import type { StudioSession } from "../uiTypes";

type SidebarProps = {
  app: StudioApp | null;
  agents: Agent[];
  selectedAgent: string;
  appID: string;
  userID: string;
  sessions: StudioSession[];
  activeSessionKey: string;
  sessionID: string;
  onSelectedAgentChange: (agentID: string) => void;
  onAppIDChange: (appID: string) => void;
  onUserIDChange: (userID: string) => void;
  onOpenCreateSession: () => void;
  onSelectSession: (sessionKey: string) => void;
  onActiveSessionIDChange: (sessionID: string) => void;
};

export function Sidebar({
  app,
  agents,
  selectedAgent,
  appID,
  userID,
  sessions,
  activeSessionKey,
  sessionID,
  onSelectedAgentChange,
  onAppIDChange,
  onUserIDChange,
  onOpenCreateSession,
  onSelectSession,
  onActiveSessionIDChange
}: SidebarProps) {
  return (
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
          <select value={selectedAgent} onChange={(event) => onSelectedAgentChange(event.target.value)}>
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
          <input value={activeSessionBackendLabel(app)} readOnly />
        </label>
        {app?.session_backend?.description ? (
          <div className="backend-panel" aria-label="Session backend">
            <strong>{app.session_backend.name}</strong>
            <p>{app.session_backend.description}</p>
          </div>
        ) : null}
      </section>

      <section className="control-section session-section">
        <div className="control-section-header">
          <h2>Sessions</h2>
          <button
            className="icon-button"
            type="button"
            title="Create session"
            aria-label="Create session"
            onClick={onOpenCreateSession}
          >
            +
          </button>
        </div>
        <label>
          App ID
          <input value={appID} onChange={(event) => onAppIDChange(event.target.value)} />
        </label>
        <label>
          User ID
          <input value={userID} onChange={(event) => onUserIDChange(event.target.value)} />
        </label>
        <div className="session-list" aria-label="Sessions">
          {sessions.map((session) => (
            <button
              key={session.key}
              type="button"
              className={`session-list-item${session.key === activeSessionKey ? " is-active" : ""}`}
              onClick={() => onSelectSession(session.key)}
            >
              <span className="session-item-title">{sessionTitle(session)}</span>
              <span className="session-item-id">{session.id || "Untitled session"}</span>
              <span className="session-item-count">{messageCountLabel(session.messages.length)}</span>
            </button>
          ))}
        </div>
        <label>
          Active Session ID
          <input value={sessionID} onChange={(event) => onActiveSessionIDChange(event.target.value)} />
        </label>
      </section>
    </aside>
  );
}

function activeSessionBackendLabel(app: StudioApp | null): string {
  if (!app) {
    return "Loading";
  }
  if (!app.has_session_service) {
    return "Not configured";
  }
  return app.session_backend?.name || "Configured";
}

function sessionTitle(session: StudioSession): string {
  return session.title || session.id || "Untitled session";
}

function messageCountLabel(count: number): string {
  if (count === 1) {
    return "1 message";
  }
  return `${count} messages`;
}
