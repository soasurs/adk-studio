import type { FormEvent } from "react";
import type { SessionDraft } from "../uiTypes";

type CreateSessionDialogProps = {
  draft: SessionDraft;
  draftID: string;
  isDuplicate: boolean;
  onDraftChange: (draft: SessionDraft) => void;
  onCreate: () => void;
  onClose: () => void;
};

export function CreateSessionDialog({
  draft,
  draftID,
  isDuplicate,
  onDraftChange,
  onCreate,
  onClose
}: CreateSessionDialogProps) {
  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onCreate();
  }

  return (
    <div className="modal-backdrop" role="presentation">
      <form className="session-dialog" aria-label="Create session" onSubmit={handleSubmit}>
        <header>
          <h2>Create Session</h2>
          <button className="ghost-icon-button" type="button" aria-label="Close dialog" onClick={onClose}>
            x
          </button>
        </header>
        <label>
          Title
          <input autoFocus value={draft.title} onChange={(event) => onDraftChange({ ...draft, title: event.target.value })} />
        </label>
        <label>
          Session ID
          <input
            value={draft.id}
            onChange={(event) => onDraftChange({ ...draft, id: event.target.value })}
            aria-invalid={isDuplicate || undefined}
          />
          {isDuplicate ? <span className="field-error">Session ID already exists.</span> : null}
        </label>
        <div className="dialog-actions">
          <button type="button" className="secondary-button" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" disabled={!draftID || isDuplicate}>
            Create
          </button>
        </div>
      </form>
    </div>
  );
}
