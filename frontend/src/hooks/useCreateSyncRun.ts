import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useCallback, useRef, useState } from "react";

import type { SyncRun } from "../api/contracts";
import { createSyncRun } from "../api/syncRuns";
import { ApiError } from "../api/problems";

export type CreateRunState =
  | { kind: "idle" }
  | { kind: "submitting" }
  // Several connections started at once: there is no single run to open, so
  // the page stays put and says what began.
  | { kind: "started"; runs: SyncRun[] }
  // scope says what the user asked for, which the 409 alone cannot: the
  // backend answers the same conflict whether every connection was busy or
  // the one connection asked for was. Without it the notice tells someone who
  // clicked "Sincronizar" on a single bank that all of them are busy.
  | { kind: "conflict"; activeRunId: string | null; scope: "all" | "one" }
  | { kind: "uncertain" };

export function useCreateSyncRun(onCreated: (id: string) => void) {
  const queryClient = useQueryClient();
  const [state, setState] = useState<CreateRunState>({ kind: "idle" });
  const locked = useRef(false);
  const mutation = useMutation({
    mutationFn: (sourceId: string | undefined) => createSyncRun(sourceId),
  });

  const submit = useCallback(
    async (sourceId?: string) => {
      if (locked.current) {
        return;
      }
      locked.current = true;
      setState({ kind: "submitting" });
      try {
        const runs = await mutation.mutateAsync(sourceId);
        await queryClient.invalidateQueries({ queryKey: ["sync-runs", "list"] });
        if (runs.length === 1) {
          onCreated(runs[0].id);
          return;
        }
        // Nothing to navigate to, so release the lock: syncing again once a
        // connection frees up is a legitimate next action.
        setState({ kind: "started", runs });
        locked.current = false;
      } catch (error) {
        if (error instanceof ApiError && error.kind === "conflict") {
          setState({
            kind: "conflict",
            activeRunId: error.problem?.active_sync_run_id ?? null,
            scope: sourceId === undefined ? "all" : "one",
          });
        } else {
          setState({ kind: "uncertain" });
        }
        locked.current = false;
      }
    },
    [mutation, onCreated, queryClient],
  );

  const reset = useCallback(() => {
    if (!locked.current) {
      setState({ kind: "idle" });
    }
  }, []);

  return { state, submit, reset };
}
