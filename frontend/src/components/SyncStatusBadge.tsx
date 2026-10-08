import type { SyncStatus } from "../api/contracts";
import { getStatusMetadata } from "../presentation/syncStatus";
import { StatusTag, type StatusTagTone } from "./shared/StatusTag";

const toneOf: Record<ReturnType<typeof getStatusMetadata>["tone"], StatusTagTone> = {
  progress: "info",
  success: "success",
  warning: "warning",
  danger: "danger",
};

/**
 * A run's status as the shared tag. Lists show it only for runs that need a
 * look (the plain "Concluída" is not tagged there); a run's own page always
 * says it.
 */
export function SyncStatusBadge({ status }: { status: SyncStatus }) {
  const metadata = getStatusMetadata(status);
  return <StatusTag tone={toneOf[metadata.tone]}>{metadata.label}</StatusTag>;
}
