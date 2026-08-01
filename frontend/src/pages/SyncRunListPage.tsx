import { PageContainer } from "@ant-design/pro-layout";
import { Empty, Flex, Typography } from "antd";

import { CreateSyncRunAction } from "../components/CreateSyncRunAction";
import { LoadingState, UnavailableState } from "../components/AsyncState";
import { SyncRunHistory } from "../components/SyncRunHistory";
import { useSyncRunList } from "../hooks/useSyncRunList";

export function SyncRunListPage() {
  const { state, retry } = useSyncRunList();

  return (
    <PageContainer
      title="Open Banking"
      subTitle="Sincronizações confirmadas pelo Contadinho"
      content="Atualize os dados da sua conexão e consulte as execuções mais recentes."
    >
      <Flex vertical gap="large">
        <CreateSyncRunAction />
        <section aria-labelledby="history-title">
          <Typography.Title id="history-title" level={2}>
            Sincronizações recentes
          </Typography.Title>
        {state.kind === "loading" && <LoadingState>Carregando histórico…</LoadingState>}
        {state.kind === "empty" && (
          <Empty
            description={
              <span>
                <strong>Nenhuma sincronização ainda</strong>
                <br />
                Use “Sincronizar agora” para iniciar a primeira atualização.
              </span>
            }
          />
        )}
        {state.kind === "unavailable" && (
          <UnavailableState onRetry={retry}>
            O histórico está temporariamente indisponível. Isso não significa que esteja vazio.
          </UnavailableState>
        )}
        {state.kind === "ready" && <SyncRunHistory runs={state.runs} />}
        </section>
      </Flex>
    </PageContainer>
  );
}
