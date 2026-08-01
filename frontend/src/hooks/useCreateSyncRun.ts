import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useCallback, useRef, useState } from "react";

import { createSyncRun } from "../api/syncRuns";
import { ApiError } from "../api/problems";

export type CreateRunState =
  | { kind: "idle" }
  | { kind: "submitting" }
  | { kind: "conflict"; activeRunId: string | null }
  | { kind: "uncertain" };

export function useCreateSyncRun(onCreated: (id: string) => void) {
  const queryClient = useQueryClient();
  const [state, setState] = useState<CreateRunState>({ kind: "idle" });
  const locked = useRef(false);
  const mutation = useMutation({
    mutationFn: () => createSyncRun(),
  });

  const submit = useCallback(async () => {
    if (locked.current) {
      return;
    }
    locked.current = true;
    setState({ kind: "submitting" });
    try {
      const run = await mutation.mutateAsync(undefined);
      await queryClient.invalidateQueries({ queryKey: ["sync-runs", "list"] });
      onCreated(run.id);
    } catch (error) {
      if (error instanceof ApiError && error.kind === "conflict") {
        setState({
          kind: "conflict",
          activeRunId: error.problem?.active_sync_run_id ?? null,
        });
      } else {
        setState({ kind: "uncertain" });
      }
      locked.current = false;
    }
  }, [mutation, onCreated, queryClient]);

  const reset = useCallback(() => {
    if (!locked.current) {
      setState({ kind: "idle" });
    }
  }, []);

  return { state, submit, reset };
}
