import { Alert, Button, Flex } from "antd";
import type { ReactNode } from "react";

import { LoadingState, UnavailableState } from "../AsyncState";
import { Page } from "./Page";

export type DetailFreshness = "loading" | "fresh" | "stale" | "not_found" | "unavailable";

/**
 * The shape every detail-page data hook already returns (useAccountDetail,
 * useInvestmentDetail, usePayableDetail, useSyncRun): a snapshot, and the
 * freshness reading that says whether it is safe to trust.
 */
export interface DetailPageQueryState<T> {
  snapshot: T | null;
  freshness: DetailFreshness;
  retrying: boolean;
}

interface DetailPageFrameProps {
  title: string;
  /** A link back to the list this record was opened from. Rendered under
   *  the title by Page, and reused as the recovery action on every error
   *  state below (not found, unavailable, invalid). */
  back: ReactNode;
  className?: string;
}

const defaultInvalidDescription = "O identificador informado não possui o formato esperado.";

/**
 * Shown instead of ever calling the page's data hook when the route's id
 * does not look like a real identifier — each page still runs that isUuid
 * check itself (calling a detail hook with a bad id would fire a real
 * request), this only gives the failure the same Page chrome as everything
 * else.
 */
export function InvalidDetailPage({
  title,
  back,
  className,
  invalidTitle,
  invalidDescription = defaultInvalidDescription,
}: DetailPageFrameProps & { invalidTitle: string; invalidDescription?: string }) {
  return (
    <Page title={title} back={back} className={className}>
      <Alert
        type="error"
        showIcon
        message={<h1>{invalidTitle}</h1>}
        description={invalidDescription}
        action={back}
      />
    </Page>
  );
}

interface DetailPageProps<T> extends DetailPageFrameProps {
  state: DetailPageQueryState<T>;
  retry: () => void;
  loadingLabel?: ReactNode;
  notFoundMessage: string;
  notFoundDescription: string;
  unavailableMessage: ReactNode;
  /** The loaded body. Only ever called with a non-null snapshot — render
   *  anything the page needs here, including modals/panels tied to it;
   *  before a snapshot exists there is nothing of the page's own state for
   *  them to show anyway. */
  children: (snapshot: T) => ReactNode;
}

/**
 * The detail-page shell shared by Conta, Investimento, Pendência and
 * Sincronização: title/back through Page, then the one
 * loading → not_found → unavailable → stale → loaded switch all four used to
 * repeat by hand. Each page still owns its own data hook, its own back link
 * and its own body (via `children`) — this only owns the chrome around
 * whatever freshness that hook reports.
 */
export function DetailPage<T>({
  title,
  back,
  className,
  state,
  retry,
  loadingLabel,
  notFoundMessage,
  notFoundDescription,
  unavailableMessage,
  children,
}: DetailPageProps<T>) {
  return (
    <Page title={title} back={back} className={className}>
      <Flex vertical gap="large">
        {state.freshness === "loading" && <LoadingState>{loadingLabel}</LoadingState>}
        {state.freshness === "not_found" && (
          <Alert
            type="error"
            showIcon
            message={notFoundMessage}
            description={notFoundDescription}
            action={back}
          />
        )}
        {state.freshness === "unavailable" && (
          <UnavailableState onRetry={retry}>{unavailableMessage}</UnavailableState>
        )}
        {state.snapshot !== null && (
          <>
            {state.freshness === "stale" && (
              <Alert
                type="warning"
                showIcon
                message="As informações podem estar desatualizadas."
                action={
                  <Button loading={state.retrying} onClick={retry}>
                    Tentar novamente
                  </Button>
                }
              />
            )}
            {children(state.snapshot)}
          </>
        )}
      </Flex>
    </Page>
  );
}
