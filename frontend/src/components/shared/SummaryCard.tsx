import type { ReactNode } from "react";

import { LoadingState, UnavailableState } from "../AsyncState";
import { WidgetCard } from "./WidgetCard";

interface SummaryCardProps {
  icon: ReactNode;
  title: string;
  extra?: ReactNode;
  isLoading: boolean;
  error: unknown;
  hasData: boolean;
  loadingLabel: string;
  errorLabel: string;
  onRetry: () => void;
  children: ReactNode;
}

/**
 * Standard shell for the small metric cards used on the Home dashboard
 * (title + icon, an optional link in the corner, and the three async
 * states — loading / unavailable / content). Centralizing this keeps summary
 * cards visually consistent without each one re-declaring the same
 * Card + LoadingState + UnavailableState wiring.
 */
export function SummaryCard({
  icon,
  title,
  extra,
  isLoading,
  error,
  hasData,
  loadingLabel,
  errorLabel,
  onRetry,
  children,
}: SummaryCardProps) {
  return (
    <WidgetCard icon={icon} title={title} extra={extra}>
      {isLoading && <LoadingState>{loadingLabel}</LoadingState>}
      {!isLoading && (error || !hasData) && (
        <UnavailableState onRetry={onRetry}>{errorLabel}</UnavailableState>
      )}
      {!isLoading && !error && hasData && children}
    </WidgetCard>
  );
}
