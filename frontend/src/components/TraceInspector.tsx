import type { RunStreamEvent } from "../types";
import { traceTimeISO, traceTimeLabel, traceTitle, traceTypeLabel } from "../traceView";

export function TraceInspector({ events }: { events: RunStreamEvent[] }) {
  return (
    <aside className="inspector" aria-label="Event inspector">
      <header>
        <h2>Trace</h2>
        <p>Agent events, tool calls, usage, and errors.</p>
      </header>
      {events.length === 0 ? (
        <div className="empty-state">
          <strong>No run selected</strong>
          <span>Run events will be listed chronologically.</span>
        </div>
      ) : (
        <div className="trace-list">
          {events.map((trace, index) => (
            <details key={`${trace.run_id}-${index}`} className="trace-item">
              <summary>
                <div className="trace-summary-main">
                  <strong>{traceTitle(trace)}</strong>
                  <div className="trace-meta">
                    <span>{traceTypeLabel(trace)}</span>
                    <time dateTime={traceTimeISO(trace)}>{traceTimeLabel(trace)}</time>
                  </div>
                </div>
              </summary>
              <pre>{JSON.stringify(trace, null, 2)}</pre>
            </details>
          ))}
        </div>
      )}
    </aside>
  );
}
