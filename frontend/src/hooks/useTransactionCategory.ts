import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";

import type { TransactionCategoryResult } from "../api/contracts";
import { setTransactionCategory } from "../api/transactions";

export interface CategoryTarget {
  transactionId: string;
  categoryId: string;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Não foi possível salvar a categoria.";
}

export interface TransactionWriteCallbacks<Target, Result> {
  /** The write was confirmed by the API (the list refresh may still be pending). */
  onSaved?: (target: Target, result: Result) => void;
  /** The write failed; `writeError` carries the message for the inline Alert. */
  onFailed?: (target: Target, message: string) => void;
}

export function useTransactionCategory({ onSaved, onFailed }: TransactionWriteCallbacks<CategoryTarget, TransactionCategoryResult> = {}) {
  const queryClient = useQueryClient();
  const locked = useRef(false);
  const [pendingTarget, setPendingTarget] = useState<CategoryTarget | null>(null);
  const [failedWrite, setFailedWrite] = useState<CategoryTarget | null>(null);
  const [writeError, setWriteError] = useState<string | null>(null);
  const [refreshError, setRefreshError] = useState<string | null>(null);
  const [announcement, setAnnouncement] = useState("");

  const refresh = async () => {
    setRefreshError(null);
    try {
      await queryClient.refetchQueries(
        { queryKey: ["transactions"], type: "active" },
        { throwOnError: true },
      );
      setAnnouncement("Categoria atualizada. Totais atualizados.");
    } catch (error) {
      setRefreshError(errorMessage(error));
      setAnnouncement("Categoria salva, atualização dos resultados pendente.");
    }
  };

  const mutation = useMutation({
    mutationFn: (target: CategoryTarget) =>
      setTransactionCategory(target.transactionId, target.categoryId),
    onMutate: async (target) => {
      setPendingTarget(target);
      setFailedWrite(null);
      setWriteError(null);
      setRefreshError(null);
      setAnnouncement("Salvando categoria da transação…");
      await queryClient.cancelQueries({ queryKey: ["transactions"] });
    },
    onSuccess: async (result, target) => {
      onSaved?.(target, result);
      await queryClient.invalidateQueries({ queryKey: ["transactions"], refetchType: "none" });
      await refresh();
    },
    onError: (error, target) => {
      setFailedWrite(target);
      setWriteError(errorMessage(error));
      onFailed?.(target, errorMessage(error));
      setAnnouncement("Falha ao salvar. A última categoria confirmada foi mantida.");
    },
    onSettled: () => {
      locked.current = false;
      setPendingTarget(null);
    },
  });

  return {
    setCategory: (target: CategoryTarget) => {
      if (locked.current) return;
      locked.current = true;
      mutation.mutate(target);
    },
    retryWrite: () => {
      if (!failedWrite || locked.current) return;
      locked.current = true;
      mutation.mutate(failedWrite);
    },
    retryRefresh: () => refresh(),
    pendingTarget,
    failedTarget: failedWrite,
    isPending: mutation.isPending,
    writeError,
    refreshError,
    announcement,
  };
}
