import { ChevronRightIcon, Maximize2Icon } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger
} from "@/components/ui/dialog";

import type { RunStreamEvent } from "../types";
import { formatTraceEvent } from "../formatDisplay";
import { traceTimeISO, traceTimeLabel, traceTitle, traceTypeLabel } from "../traceView";

const tracePreviewCharacterLimit = 2600;
const tracePreviewLineLimit = 80;

export function TraceInspector({ events }: { events: RunStreamEvent[] }) {
  return (
    <aside className="hidden h-dvh min-h-0 flex-col overflow-hidden border-l bg-sidebar p-5 xl:flex" aria-label="Event inspector">
      <header className="flex shrink-0 items-center justify-between gap-3">
        <div>
          <h2 className="text-base font-semibold leading-none">Trace</h2>
          <p className="mt-1 text-sm text-muted-foreground">{events.length} events</p>
        </div>
      </header>

      {events.length === 0 ? (
        <Card className="mt-5 border-dashed">
          <CardContent className="grid gap-1 p-4">
            <strong className="text-sm font-semibold">No trace events</strong>
            <span className="text-sm text-muted-foreground">Run events will appear here.</span>
          </CardContent>
        </Card>
      ) : (
        <div className="mt-5 flex min-h-0 flex-1 basis-0 flex-col gap-2 overflow-y-auto overflow-x-hidden pr-1">
          {events.map((trace, index) => (
            <TraceEventCard key={`${trace.run_id}-${index}`} trace={trace} />
          ))}
        </div>
      )}
    </aside>
  );
}

function TraceEventCard({ trace }: { trace: RunStreamEvent }) {
  const content = formatTraceEvent(trace);
  const isLong = isLongTraceContent(content);
  const preview = isLong ? tracePreview(content) : content;

  return (
    <Card className="shrink-0 overflow-hidden">
      <details className="group">
        <summary className="flex min-h-18 cursor-pointer list-none items-center gap-2 p-3 [&::-webkit-details-marker]:hidden">
          <ChevronRightIcon className="size-4 shrink-0 text-muted-foreground transition-transform group-open:rotate-90" />
          <div className="grid min-w-0 flex-1 gap-2">
            <strong className="min-w-0 truncate text-sm font-medium">{traceTitle(trace)}</strong>
            <div className="flex min-w-0 items-center gap-2">
              <Badge variant="secondary" className="uppercase">
                {traceTypeLabel(trace)}
              </Badge>
              <Badge variant="outline" className="font-mono">
                <time dateTime={traceTimeISO(trace)}>{traceTimeLabel(trace)}</time>
              </Badge>
            </div>
          </div>
        </summary>
        <div className="border-t bg-muted/45">
          <pre className="max-h-96 overflow-auto p-3 font-mono text-xs leading-5 whitespace-pre-wrap text-foreground">{preview}</pre>
          {isLong ? (
            <div className="flex justify-end border-t bg-card/70 px-3 py-2">
              <Dialog>
                <DialogTrigger asChild>
                  <Button type="button" variant="outline" size="sm">
                    <Maximize2Icon />
                    View full
                  </Button>
                </DialogTrigger>
                <DialogContent className="grid max-h-[90dvh] max-w-5xl grid-rows-[auto_minmax(0,1fr)] gap-0 overflow-hidden p-0">
                  <DialogHeader className="border-b px-5 pt-5 pb-4">
                    <DialogTitle>{traceTitle(trace)}</DialogTitle>
                    <DialogDescription>
                      {traceTypeLabel(trace)} at {traceTimeLabel(trace)}
                    </DialogDescription>
                  </DialogHeader>
                  <pre className="min-h-0 overflow-auto bg-muted/45 p-4 font-mono text-xs leading-5 whitespace-pre text-foreground">{content}</pre>
                </DialogContent>
              </Dialog>
            </div>
          ) : null}
        </div>
      </details>
    </Card>
  );
}

function isLongTraceContent(content: string): boolean {
  return content.length > tracePreviewCharacterLimit || content.split("\n").length > tracePreviewLineLimit;
}

function tracePreview(content: string): string {
  const lines = content.split("\n");
  let preview = lines.length > tracePreviewLineLimit ? lines.slice(0, tracePreviewLineLimit).join("\n") : content;
  if (preview.length > tracePreviewCharacterLimit) {
    preview = preview.slice(0, tracePreviewCharacterLimit);
  }
  return `${preview.trimEnd()}\n...`;
}
