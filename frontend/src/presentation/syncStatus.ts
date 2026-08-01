import type { FailureStage, SyncStatus } from "../api/contracts";

const statusMetadata = {
  in_progress: { label: "Em andamento", symbol: "↻", tone: "progress" },
  completed: { label: "Concluída", symbol: "✓", tone: "success" },
  completed_with_failures: {
    label: "Concluída com falhas",
    symbol: "!",
    tone: "warning",
  },
  failed: { label: "Falhou", symbol: "×", tone: "danger" },
} as const satisfies Record<SyncStatus, { label: string; symbol: string; tone: string }>;

const stageLabels = {
  auth: "Autenticação",
  item: "Conexão",
  accounts: "Listagem de contas",
  account: "Conta",
  transactions: "Transações",
  normalize: "Normalização",
  interrupted: "Interrupção",
  worker_unavailable: "Processamento indisponível",
} as const satisfies Record<FailureStage, string>;

export const getStatusMetadata = (status: SyncStatus) => statusMetadata[status];
export const getFailureStageLabel = (stage: FailureStage) => stageLabels[stage];
export const isFinalStatus = (status: SyncStatus): boolean => status !== "in_progress";
