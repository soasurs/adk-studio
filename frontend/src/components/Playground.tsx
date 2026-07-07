import type { KeyboardEvent, RefObject } from "react";
import { AlertCircleIcon, LoaderCircleIcon, PlayIcon, SendIcon } from "lucide-react";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";

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
    <section className="flex min-h-0 min-w-0 flex-col overflow-hidden p-4 md:h-dvh md:p-5" aria-label="Playground">
      <header className="flex items-center justify-between gap-4 border-b pb-4">
        <div className="min-w-0">
          <h2 className="text-base font-semibold leading-none">Playground</h2>
          <p className="mt-1 truncate font-mono text-xs text-muted-foreground">{sessionID || "No active session"}</p>
        </div>
        <Button type="button" onClick={onRun} disabled={runDisabled}>
          {isRunning ? <LoaderCircleIcon className="animate-spin" /> : <PlayIcon />}
          {isRunning ? "Running" : "Run"}
        </Button>
      </header>

      <div className="grid min-h-0 flex-1 content-start gap-3 overflow-auto py-5" ref={messageListRef}>
        {messages.length === 0 ? (
          <Card className="max-w-3xl border-l-[3px] border-l-amber-500">
            <CardContent className="grid gap-2 p-4">
              <Badge variant="secondary" className="w-fit">
                Studio
              </Badge>
              <p className="text-sm text-muted-foreground">No messages yet.</p>
            </CardContent>
          </Card>
        ) : null}
        {messages.map((message) => (
          <MessageItem key={message.id} message={message} />
        ))}
      </div>

      {error ? (
        <Alert variant="destructive" className="mb-3 flex items-start gap-2">
          <AlertCircleIcon className="mt-0.5 size-4 shrink-0" />
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : null}

      <form
        className="grid gap-3 border-t pt-4"
        onSubmit={(event) => {
          event.preventDefault();
          onRun();
        }}
      >
        <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto]">
          <Textarea
            placeholder="Message ADK runner"
            rows={2}
            value={input}
            onChange={(event) => onInputChange(event.target.value)}
            onKeyDown={onComposerKeyDown}
            className="h-[58px] min-h-[58px] resize-none overflow-auto leading-5"
          />
          <Button type="submit" disabled={runDisabled} className="h-[58px] min-w-22">
            {isRunning ? <LoaderCircleIcon className="animate-spin" /> : <SendIcon />}
            {isRunning ? "Sending" : "Send"}
          </Button>
        </div>
        <div className="flex min-h-8 flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <Switch
              id="streaming-enabled"
              checked={streamingEnabled}
              disabled={isRunning}
              onCheckedChange={onStreamingEnabledChange}
            />
            <Label htmlFor="streaming-enabled" className="text-sm text-muted-foreground">
              Streaming
            </Label>
          </div>
          <div className="flex items-center gap-3">
            <Separator orientation="vertical" className="hidden h-5 sm:block" />
            <ToggleGroup
              type="single"
              value={sendShortcut}
              onValueChange={(value) => {
                if (value) {
                  onSendShortcutChange(value as SendShortcut);
                }
              }}
              aria-label="Send shortcut"
            >
              <ToggleGroupItem value="enter" size="sm" aria-label="Send with Enter">
                Enter
              </ToggleGroupItem>
              <ToggleGroupItem value="modified" size="sm" aria-label="Send with modifier Enter">
                Modified Enter
              </ToggleGroupItem>
            </ToggleGroup>
          </div>
        </div>
      </form>
    </section>
  );
}
