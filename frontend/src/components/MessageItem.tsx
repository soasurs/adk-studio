import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";

import type { Message } from "../types";
import { MarkdownContent } from "./MarkdownContent";

const roleStyles: Record<Message["role"], string> = {
  assistant: "border-l-amber-500",
  user: "border-l-primary",
  tool_call: "border-l-yellow-600 bg-yellow-50/60",
  tool_result: "border-l-blue-500 bg-blue-50/60",
  system: "border-l-slate-500",
  error: "border-l-destructive"
};

export function MessageItem({ message }: { message: Message }) {
  const timestamp = messageTimestamp(message.createdAt);

  return (
    <article className="max-w-3xl" aria-busy={message.partial || undefined}>
      <Card
        className={cn(
          "border-l-[3px]",
          roleStyles[message.role],
          message.partial && "border-dashed bg-card/70"
        )}
      >
        <CardContent className="grid gap-3 p-4">
          <div className="flex min-w-0 items-center justify-between gap-3">
            <Badge variant={message.role === "error" ? "destructive" : "secondary"} className="min-w-0 max-w-full truncate">
              {message.author}
            </Badge>
            {timestamp ? (
              <time
                dateTime={timestamp.iso}
                title={timestamp.full}
                className="shrink-0 font-mono text-xs tabular-nums text-muted-foreground"
              >
                {timestamp.label}
              </time>
            ) : null}
          </div>
          {message.reasoning ? (
            <details className="rounded-md border border-l-[3px] border-l-muted-foreground/60 bg-muted/50 p-3" open>
              <summary className="cursor-pointer text-xs font-semibold uppercase text-muted-foreground">
                Reasoning
              </summary>
              <pre className="mt-2 max-h-72 overflow-auto whitespace-pre-wrap font-mono text-xs leading-5 text-muted-foreground">
                {message.reasoning}
              </pre>
            </details>
          ) : null}
          {message.content ? (
            <div className="grid gap-2">
              {message.reasoning ? (
                <Badge variant="outline" className="w-fit">
                  Response
                </Badge>
              ) : null}
              <MarkdownContent content={message.content} />
            </div>
          ) : null}
        </CardContent>
      </Card>
    </article>
  );
}

function messageTimestamp(timestamp?: number): { full: string; iso: string; label: string } | null {
  if (timestamp === undefined) {
    return null;
  }
  const date = new Date(timestamp);
  if (Number.isNaN(date.getTime())) {
    return null;
  }
  return {
    full: date.toLocaleString(),
    iso: date.toISOString(),
    label: date.toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit"
    })
  };
}
