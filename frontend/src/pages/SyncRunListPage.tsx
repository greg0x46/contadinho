import { PluggySettings } from "../components/PluggySettings";
import { SettingsPageContainer as PageContainer } from "../components/SettingsPageContainer";
import { Collapse, Empty } from "antd";
import { useNavigate } from "react-router-dom";

import { CreateSyncRunAction } from "../components/CreateSyncRunAction";
import { DataSourceConnections } from "../components/DataSourceConnections";
import { LoadingState, UnavailableState } from "../components/AsyncState";
import { DataCard, SectionHeader } from "../components/layout";
import { SyncRunHistory, SyncRunHistorySummary } from "../components/SyncRunHistory";
import { useCreateSyncRun } from "../hooks/useCreateSyncRun";
import { useSyncRunList } from "../hooks/useSyncRunList";

export function SyncRunListPage() {
  const { state, retry } = useSyncRunList();
  const navigate = useNavigate();
  const sync = useCreateSyncRun((id) => navigate(`/configuracoes/open-banking/sync-runs/${id}`));

  return (
    <PageContainer
      title="Open Banking"
      subTitle="Sincronizações confirmadas pelo Contadinho"
      content="Atualize os dados das suas conexões e consulte as execuções mais recentes."
    >
      <div className="sync-runs-page">
        <CreateSyncRunAction sync={sync} />
        <DataSourceConnections sync={sync} />
        <section aria-label="Sincronizações recentes">
          <SectionHeader title="Sincronizações recentes" />
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
          {state.kind === "ready" && (
            <DataCard flush summary={<SyncRunHistorySummary runs={state.runs} />}>
              <SyncRunHistory runs={state.runs} />
            </DataCard>
          )}
        </section>
        <Collapse items={[{ key: "credentials", label: "Credenciais da Pluggy", children: <PluggySettings /> }]} />
      </div>
    </PageContainer>
  );
}
