import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import type { Category, ManualTransactionWrite, TransactionItem } from "../../api/contracts";
import { useManualTransaction } from "../../hooks/useManualTransaction";
import { useTransactionCategory } from "../../hooks/useTransactionCategory";
import { useTransactionInclusion } from "../../hooks/useTransactionInclusion";
import { manualTransactionErrorMessage } from "../../presentation/manualTransactionErrors";
import { useFeedback } from "../shared/useFeedback";
import { RetryAlert } from "./RetryAlert";
import type { TransactionPanelProps } from "./TransactionPanel";

/** A failed write, ready to render inside the panel next to its control. */
export interface WriteIssue {
  message: string;
  retry: () => void;
}

type PanelWriteProps = Pick<
  TransactionPanelProps,
  | "onInclusion"
  | "inclusionPending"
  | "inclusionError"
  | "onCategory"
  | "categoryPending"
  | "categoryError"
  | "onSaveManual"
  | "saveManualPending"
  | "onDeleteManual"
  | "deleteManualPending"
  | "deleteManualError"
>;

/**
 * Everything the transaction panel writes — category, "considerar nos
 * totais", editing and deleting a manual transaction — wired once for both
 * pages that open it (Transações and the account detail).
 *
 * Feedback lives where the person is: a short toast confirms a write, and a
 * failure is an inline Alert with a retry inside the panel (see
 * `panelProps`), or on the page when the write came from a row's menu while
 * no panel was open (see `alerts`). No "Desfazer": there is no endpoint to
 * undo a categorization.
 */
export function useTransactionPanelWrites({
  categories,
  onDeleted,
  onConfirmed,
  panelOpen,
}: {
  categories: Category[];
  /**
   * Whether a transaction panel is open. When it is, a failed write is
   * already an inline Alert inside it, so no error toast repeats it (on a
   * phone that toast would cover the panel's title).
   */
  panelOpen: boolean;
  /** The deleted line's panel has nothing left to show. */
  onDeleted: () => void;
  /** A confirmed write, as the line it produced (see `useSelectedTransaction().patch`). */
  onConfirmed: (id: string, update: (item: TransactionItem) => TransactionItem) => void;
}) {
  const feedback = useFeedback();
  const panelOpenRef = useRef(panelOpen);
  useEffect(() => {
    panelOpenRef.current = panelOpen;
  }, [panelOpen]);
  const failed = (message: string) => {
    if (!panelOpenRef.current) feedback.error(message);
  };
  const categoriesById = useMemo(
    () => new Map(categories.map((category) => [category.id, category])),
    [categories],
  );
  const inclusion = useTransactionInclusion({
    onSaved: ({ transactionId, state }, result) => {
      feedback.success(state === "ignored" ? "Transação ignorada" : "Transação considerada nos totais");
      onConfirmed(transactionId, (item) => ({
        ...item,
        inclusion: { state, changed_at: result.changed_at, origin: "manual", rule_name: null },
        totals_eligibility:
          state === "ignored"
            ? { included: false, reason: "ignored" }
            : item.totals_eligibility.reason === "ignored"
              ? { included: true, reason: null }
              : item.totals_eligibility,
      }));
    },
    onFailed: () => failed("Não foi possível salvar a decisão."),
  });
  const category = useTransactionCategory({
    onSaved: ({ transactionId, categoryId }, result) => {
      const chosen = categoriesById.get(categoryId);
      feedback.success(chosen ? `Categoria alterada para ${chosen.name}` : "Categoria alterada");
      if (chosen) {
        onConfirmed(transactionId, (item) => ({
          ...item,
          internal_category: {
            id: chosen.id,
            name: chosen.name,
            kind: chosen.kind,
            is_active: chosen.is_active,
            icon: chosen.icon,
            color: chosen.color,
            origin: result.origin,
            changed_at: result.changed_at,
          },
        }));
      }
    },
    onFailed: () => failed("Não foi possível salvar a categoria."),
  });
  const manualTransaction = useManualTransaction();
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const saveManual = async (transactionId: string, write: ManualTransactionWrite) => {
    try {
      await manualTransaction.update({ transactionId, write });
      feedback.success("Transação salva");
    } catch (error) {
      throw new Error(manualTransactionErrorMessage(error, "save"));
    }
  };
  const deleteManual = async (transactionId: string) => {
    setDeleteError(null);
    try {
      await manualTransaction.remove(transactionId);
      feedback.success("Transação excluída");
      onDeleted();
    } catch (error) {
      setDeleteError(manualTransactionErrorMessage(error, "delete"));
    }
  };

  // A failure belongs to the line it happened on: opening another line must
  // not greet it with an unrelated error.
  const issue = (
    hook: { writeError: string | null; retryWrite: () => void; failedTarget: { transactionId: string } | null },
    selected: TransactionItem | null,
  ): WriteIssue | null =>
    hook.writeError === null || hook.failedTarget?.transactionId !== selected?.id
      ? null
      : { message: hook.writeError, retry: hook.retryWrite };

  /** The write-related props of `TransactionPanel` for the currently selected line. */
  const panelProps = (selected: TransactionItem | null): PanelWriteProps => ({
    onInclusion: (transactionId, state) => inclusion.setInclusion({ transactionId, state }),
    inclusionPending: inclusion.pendingTarget?.transactionId === selected?.id,
    inclusionError: issue(inclusion, selected),
    onCategory: (transactionId, categoryId) => category.setCategory({ transactionId, categoryId }),
    categoryPending: category.pendingTarget?.transactionId === selected?.id,
    categoryError: issue(category, selected),
    onSaveManual: saveManual,
    saveManualPending: manualTransaction.isUpdating,
    onDeleteManual: deleteManual,
    deleteManualPending: manualTransaction.isRemoving,
    deleteManualError: deleteError,
  });

  /**
   * The page-level view of the same writes. A write error only shows here
   * while no panel is open — otherwise the panel shows it, and this Alert
   * would sit unseen behind it. "Saved, refresh pending" always shows: it is
   * about the list, not about the panel's controls.
   */
  const alerts = (panelOpen: boolean): ReactNode => (
    <>
      {!panelOpen && inclusion.writeError && (
        <RetryAlert
          type="error"
          message="Não foi possível salvar a decisão"
          detail={`O último estado confirmado foi mantido. ${inclusion.writeError}`}
          action="Tentar novamente"
          onAction={inclusion.retryWrite}
        />
      )}
      {inclusion.refreshError && (
        <RetryAlert
          type="warning"
          message="Alteração salva, atualização pendente"
          detail={`Os resultados anteriores foram preservados. ${inclusion.refreshError}`}
          action="Atualizar resultados"
          onAction={inclusion.retryRefresh}
        />
      )}
      {!panelOpen && category.writeError && (
        <RetryAlert
          type="error"
          message="Não foi possível salvar a categoria"
          detail={`A última categoria confirmada foi mantida. ${category.writeError}`}
          action="Tentar novamente"
          onAction={category.retryWrite}
        />
      )}
      {category.refreshError && (
        <RetryAlert
          type="warning"
          message="Categoria salva, atualização pendente"
          detail={`Os resultados anteriores foram preservados. ${category.refreshError}`}
          action="Atualizar resultados"
          onAction={category.retryRefresh}
        />
      )}
      <div className="visually-hidden" aria-live="polite" aria-atomic="true">
        {inclusion.announcement}
      </div>
      <div className="visually-hidden" aria-live="polite" aria-atomic="true">
        {category.announcement}
      </div>
    </>
  );

  return {
    inclusion,
    panelProps,
    alerts,
    manualTransaction,
    clearDeleteError: () => setDeleteError(null),
  };
}
