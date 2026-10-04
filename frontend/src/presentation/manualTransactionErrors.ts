import { ApiError } from "../api/problems";

/**
 * The API refuses to edit or delete a manual bank line that still has parcels
 * linked to aportes/resgates, because rewriting it would silently change what
 * the investment ledger was reconciled against. The generic problem detail
 * does not say where to undo those links, so this spells the next step out.
 */
const investmentLinkedProblem = "/problems/investment-linked-transaction";

export function isInvestmentLinkedProblem(error: unknown): boolean {
  return error instanceof ApiError && error.problem?.type === investmentLinkedProblem;
}

export function manualTransactionErrorMessage(error: unknown, action: "save" | "delete"): string {
  if (isInvestmentLinkedProblem(error)) {
    return action === "delete"
      ? "Esta transação tem parcelas vinculadas a aportes ou resgates. Desfaça os vínculos em “Vincular a investimento”, no detalhe da transação, antes de excluí-la."
      : "Esta transação tem parcelas vinculadas a aportes ou resgates. Desfaça os vínculos em “Vincular a investimento”, no detalhe da transação, antes de editá-la.";
  }
  if (error instanceof Error) return error.message;
  return action === "delete"
    ? "Não foi possível excluir a transação manual."
    : "Não foi possível salvar a transação manual.";
}
