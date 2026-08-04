import type { ReceivableStatus } from "../api/contracts";

export const receivableStatusLabel: Record<ReceivableStatus, string> = {
  open: "Aberta",
  settled: "Recebida",
};

export const receivableStatusColor: Record<ReceivableStatus, string> = {
  open: "processing",
  settled: "success",
};
