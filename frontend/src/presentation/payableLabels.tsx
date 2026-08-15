import { DollarOutlined, WalletOutlined } from "@ant-design/icons";
import type { ReactNode } from "react";

import type { PayableKind, PayableStatus } from "../api/contracts";

export const payableStatusLabel: Record<PayableKind, Record<PayableStatus, string>> = {
  debt: { open: "Aberta", settled: "Quitada" },
  receivable: { open: "Aberta", settled: "Recebida" },
};

export const payableStatusColor: Record<PayableStatus, string> = {
  open: "processing",
  settled: "success",
};

// payableVocabulary centralizes the one directional difference between a
// debt ("pagamento" wording) and a receivable ("recebimento" wording) — see
// PayableTimeline, PayableForm, PayableHeaderCard, PayableList,
// PayablesSummary — so those components read the right copy for whichever
// kind they're rendering instead of existing as two near-duplicate files.
export interface PayableVocabulary {
  icon: ReactNode;
  ariaLabel: string;
  settledColumnLabel: string;
  settledCountLabel: string;
  remainingSuffix: string;
  planNoun: string;
  onTrackMessage: string;
  noPlanMessage: string;
  createPlanButtonText: string;
  generateFromRemainingText: string;
  addActionLabel: string;
  deallocateDescription: string;
  newTitle: string;
  editTitle: string;
  nameFieldPlaceholder: string;
  nameRequiredError: string;
  deleteTitle: string;
  summaryTitle: string;
  summaryCaption: string;
  emptyListText: string;
}

export const payableVocabulary: Record<PayableKind, PayableVocabulary> = {
  debt: {
    icon: <WalletOutlined aria-hidden="true" />,
    ariaLabel: "Dívidas",
    settledColumnLabel: "Pago",
    settledCountLabel: "Quitadas",
    remainingSuffix: "restantes",
    planNoun: "plano de pagamento",
    onTrackMessage: "Em dia com o plano de pagamento até hoje.",
    noPlanMessage: "Esta dívida ainda não tem um plano de pagamento.",
    createPlanButtonText: "Criar plano de pagamento",
    generateFromRemainingText: "Gerar parcelas a partir do valor restante da dívida",
    addActionLabel: "Adicionar pagamento",
    deallocateDescription:
      "A parcela deixa de contar esse valor como pago; a transação em si permanece vinculada à dívida.",
    newTitle: "Nova dívida",
    editTitle: "Editar dívida",
    nameFieldPlaceholder: "Ex.: Financiamento do carro",
    nameRequiredError: "Informe um nome para a dívida.",
    deleteTitle: "Excluir dívida",
    summaryTitle: "Resumo das dívidas",
    summaryCaption: "restantes em dívidas abertas",
    emptyListText: "Nenhuma dívida cadastrada ainda.",
  },
  receivable: {
    icon: <DollarOutlined aria-hidden="true" />,
    ariaLabel: "Contas a receber",
    settledColumnLabel: "Recebido",
    settledCountLabel: "Recebidas",
    remainingSuffix: "a receber",
    planNoun: "plano de recebimento",
    onTrackMessage: "Em dia com o plano de recebimento até hoje.",
    noPlanMessage: "Esta conta a receber ainda não tem um plano de recebimento.",
    createPlanButtonText: "Criar plano de recebimento",
    generateFromRemainingText: "Gerar parcelas a partir do valor restante da conta a receber",
    addActionLabel: "Adicionar recebimento",
    deallocateDescription:
      "A parcela deixa de contar esse valor como recebido; a transação em si permanece vinculada à conta a receber.",
    newTitle: "Nova conta a receber",
    editTitle: "Editar conta a receber",
    nameFieldPlaceholder: "Ex.: Empréstimo para Ana",
    nameRequiredError: "Informe um nome para a conta a receber.",
    deleteTitle: "Excluir conta a receber",
    summaryTitle: "Resumo das contas a receber",
    summaryCaption: "a receber em contas abertas",
    emptyListText: "Nenhuma conta a receber cadastrada ainda.",
  },
};
