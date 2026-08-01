import { PageContainer } from "@ant-design/pro-layout";
import { Alert, Button, Flex } from "antd";
import { Link, useParams } from "react-router-dom";

import { isUuid } from "../api/contracts";
import { LoadingState, UnavailableState } from "../components/AsyncState";
import { SyncFailureList } from "../components/SyncFailureList";
import { SyncRunOverview } from "../components/SyncRunOverview";
import { useSyncRun } from "../hooks/useSyncRun";

function InvalidRun() {
  return (
    <Alert
      type="error"
      showIcon
      message={<h1>Endereço de sincronização inválido</h1>}
      description="O identificador informado não possui o formato esperado."
      action={<Link to="/open-banking">Voltar para sincronizações</Link>}
    />
  );
}

function ValidRunDetail({ id }: { id: string }) {
  const { state, retry } = useSyncRun(id);
  return (
    <PageContainer
      title="Detalhes da sincronização"
      extra={<Link to="/open-banking">Voltar para sincronizações</Link>}
    >
      <Flex vertical gap="large">
      {state.freshness === "loading" && <LoadingState>Carregando sincronização…</LoadingState>}
      {state.freshness === "not_found" && (
          <Alert
            type="error"
            showIcon
            message="Sincronização não encontrada"
            description="Não existe uma execução com este identificador."
            action={<Link to="/open-banking">Voltar para sincronizações</Link>}
          />
      )}
      {state.freshness === "unavailable" && (
        <UnavailableState onRetry={retry}>
          Não foi possível consultar esta sincronização agora.
        </UnavailableState>
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
          <SyncRunOverview run={state.snapshot} />
          <SyncFailureList failures={state.snapshot.failures} />
        </>
      )}
      </Flex>
    </PageContainer>
  );
}

export function SyncRunDetailPage() {
  const { id = "" } = useParams();
  return isUuid(id) ? <ValidRunDetail id={id} /> : <InvalidRun />;
}
