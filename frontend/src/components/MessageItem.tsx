import type { Message } from "../types";
import { MarkdownContent } from "./MarkdownContent";

export function MessageItem({ message }: { message: Message }) {
  return (
    <article className={messageClassName(message)} aria-busy={message.partial || undefined}>
      <span>{message.author}</span>
      {message.reasoning ? (
        <details className="reasoning-block" open>
          <summary>Reasoning</summary>
          <pre>{message.reasoning}</pre>
        </details>
      ) : null}
      {message.content ? (
        <div className="response-block">
          {message.reasoning ? <span>Response</span> : null}
          <MarkdownContent content={message.content} />
        </div>
      ) : null}
    </article>
  );
}

function messageClassName(message: Message): string {
  const partialClass = message.partial ? " is-partial" : "";
  return `message ${message.role}-message${partialClass}`;
}
