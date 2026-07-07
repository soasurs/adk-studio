import type { FormEvent } from "react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

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
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) {
          onClose();
        }
      }}
    >
      <DialogContent aria-label="Create session">
        <form className="grid gap-4" onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Create Session</DialogTitle>
          </DialogHeader>
          <div className="grid gap-2">
            <Label htmlFor="new-session-title">Title</Label>
            <Input
              id="new-session-title"
              autoFocus
              value={draft.title}
              onChange={(event) => onDraftChange({ ...draft, title: event.target.value })}
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="new-session-id">Session ID</Label>
            <Input
              id="new-session-id"
              value={draft.id}
              onChange={(event) => onDraftChange({ ...draft, id: event.target.value })}
              aria-invalid={isDuplicate || undefined}
            />
            {isDuplicate ? <span className="text-xs text-destructive">Session ID already exists.</span> : null}
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={!draftID || isDuplicate}>
              Create
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
