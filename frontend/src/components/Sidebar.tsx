import type { ReactNode } from "react";
import { BotIcon, DatabaseIcon, PlusIcon } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { cn } from "@/lib/utils";

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
    <aside
      className="flex min-h-0 max-md:max-h-[45dvh] flex-col overflow-hidden border-b bg-sidebar text-sidebar-foreground md:h-dvh md:border-r md:border-b-0"
      aria-label="Project controls"
    >
      <div className="flex items-center gap-3 px-4 py-4 md:px-5">
        <span className="grid size-11 shrink-0 place-items-center rounded-lg bg-primary text-xs font-semibold text-primary-foreground">
          ADK
        </span>
        <div className="min-w-0">
          <h1 className="truncate text-lg font-semibold leading-tight">ADK Studio</h1>
          <p className="truncate text-sm text-muted-foreground">{app?.name || "Embedded agent debugger"}</p>
        </div>
      </div>

      <Separator />

      <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-hidden px-4 py-4 md:px-5">
        <section className="grid gap-3" aria-labelledby="app-controls-heading">
          <h2 id="app-controls-heading" className="text-xs font-semibold uppercase text-muted-foreground">
            App
          </h2>
          <Field label="Name" htmlFor="studio-app-name">
            <Input id="studio-app-name" value={app?.name || "Loading"} readOnly className="bg-muted/50" />
          </Field>
          <Field label="Agents" htmlFor="studio-agent-select">
            <Select
              value={selectedAgent || undefined}
              onValueChange={onSelectedAgentChange}
              disabled={agents.length === 0}
            >
              <SelectTrigger id="studio-agent-select">
                <SelectValue placeholder="No agents registered" />
              </SelectTrigger>
              <SelectContent>
                {agents.map((agent) => (
                  <SelectItem key={agent.id} value={agent.id}>
                    <span className="flex min-w-0 items-center gap-2">
                      <BotIcon className="size-3.5 shrink-0 text-muted-foreground" />
                      <span className="truncate">{agent.name}</span>
                    </span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
          <Field label="Session Store" htmlFor="studio-session-store">
            <Input id="studio-session-store" value={activeSessionBackendLabel(app)} readOnly className="bg-muted/50" />
          </Field>
          {app?.session_backend?.description ? (
            <Card aria-label="Session backend" className="bg-card/80">
              <CardContent className="grid gap-1 p-3">
                <div className="flex min-w-0 items-center gap-2">
                  <DatabaseIcon className="size-4 shrink-0 text-primary" />
                  <strong className="min-w-0 truncate text-sm font-semibold">{app.session_backend.name}</strong>
                </div>
                <p className="text-xs leading-5 text-muted-foreground">{app.session_backend.description}</p>
              </CardContent>
            </Card>
          ) : null}
        </section>

        <Separator />

        <section className="flex min-h-0 flex-1 flex-col gap-3" aria-labelledby="sessions-heading">
          <div className="flex items-center justify-between gap-3">
            <h2 id="sessions-heading" className="text-xs font-semibold uppercase text-muted-foreground">
              Sessions
            </h2>
            <Button type="button" size="icon" variant="outline" aria-label="Create session" onClick={onOpenCreateSession}>
              <PlusIcon />
            </Button>
          </div>
          <Field label="App ID" htmlFor="studio-app-id">
            <Input id="studio-app-id" value={appID} onChange={(event) => onAppIDChange(event.target.value)} />
          </Field>
          <Field label="User ID" htmlFor="studio-user-id">
            <Input id="studio-user-id" value={userID} onChange={(event) => onUserIDChange(event.target.value)} />
          </Field>
          <div className="grid min-h-24 flex-1 content-start gap-2 overflow-auto pr-1" aria-label="Sessions">
            {sessions.map((session) => {
              const isActive = session.key === activeSessionKey;

              return (
                <Button
                  key={session.key}
                  type="button"
                  variant="ghost"
                  className={cn(
                    "grid h-auto w-full grid-cols-[minmax(0,1fr)_auto] items-start justify-start gap-x-3 gap-y-1 rounded-md border border-l-4 border-transparent px-3 py-2 text-left hover:border-border hover:bg-accent",
                    isActive && "border-border border-l-primary bg-accent"
                  )}
                  aria-pressed={isActive}
                  onClick={() => onSelectSession(session.key)}
                >
                  <span className="min-w-0 truncate text-sm font-semibold text-foreground">{sessionTitle(session)}</span>
                  <Badge variant="secondary" className="row-start-1 text-[11px]">
                    {messageCountLabel(session.messages.length)}
                  </Badge>
                  <span className="col-span-2 min-w-0 truncate font-mono text-xs text-muted-foreground">
                    {session.id || "Untitled session"}
                  </span>
                </Button>
              );
            })}
          </div>
          <Field label="Active Session ID" htmlFor="studio-active-session-id">
            <Input
              id="studio-active-session-id"
              value={sessionID}
              onChange={(event) => onActiveSessionIDChange(event.target.value)}
            />
          </Field>
        </section>
      </div>
    </aside>
  );
}

function Field({ label, htmlFor, children }: { label: string; htmlFor: string; children: ReactNode }) {
  return (
    <div className="grid gap-2">
      <Label htmlFor={htmlFor}>{label}</Label>
      {children}
    </div>
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
