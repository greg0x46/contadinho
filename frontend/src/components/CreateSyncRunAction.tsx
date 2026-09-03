import { Alert, Button, Card, Flex, Typography } from "antd";
import { SyncOutlined } from "@ant-design/icons";
import { Link } from "react-router-dom";

import type { useCreateSyncRun } from "../hooks/useCreateSyncRun";

// The sync state is owned by the page and shared with the connection list, so
// one lock and one notice cover both "refresh everything" and "refresh this
// bank" rather than the two disagreeing about what is running.
export function CreateSyncRunAction({ sync }: { sync: ReturnType<typeof useCreateSyncRun> }) {
  const { state, submit, reset } = sync;

  return (
    <Card>
      <Flex vertical gap="middle">
        <div>
          <Typography.Title id="create-title" level={2}>
            Atualizar dados
          </Typography.Title>
          <Typography.Paragraph>
            Busque agora os dados mais recentes disponíveis em todas as conexões ativas.
          </Typography.Paragraph>
        </div>
        <Button
          type="primary"
          aria-label="Sincronizar agora"
          icon={<SyncOutlined aria-hidden />}
          loading={state.kind === "submitting"}
          onClick={() => void submit()}
        >
          Sincronizar agora
        </Button>
        <div aria-live="polite" aria-atomic="true">
        {state.kind === "submitting" && <p role="status">Solicitando a sincronização…</p>}
        {state.kind === "started" && (
          <Alert
            type="success"
            showIcon
            message={`Sincronização iniciada em ${state.runs.length} conexões.`}
            description={
              <Flex vertical align="start" gap="small">
                {state.runs.map((run) => (
                  <Link key={run.id} to={`/open-banking/sync-runs/${run.id}`}>
                    Acompanhar {run.source_name}
                  </Link>
                ))}
                <Button type="link" onClick={reset}>
                  Fechar aviso
                </Button>
              </Flex>
            }
          />
        )}
        {state.kind === "conflict" && (
          <Alert
            type="warning"
            showIcon
            message={
              state.scope === "one"
                ? "Essa conexão já está sincronizando."
                : "Todas as conexões já estão sincronizando."
            }
            description={
              <Flex vertical align="start" gap="small">
                {state.activeRunId !== null && (
                  <Link to={`/open-banking/sync-runs/${state.activeRunId}`}>
                    Acompanhar sincronização ativa
                  </Link>
                )}
                <Button type="link" onClick={reset}>
                  Fechar aviso
                </Button>
              </Flex>
            }
          />
        )}
        {state.kind === "uncertain" && (
          <Alert
            type="error"
            showIcon
            message="Não foi possível confirmar se a sincronização começou."
            description={
              <Flex vertical align="start" gap="small">
                <span>Consulte o histórico antes de tentar novamente.</span>
                <Button onClick={reset}>Tentar novamente</Button>
              </Flex>
            }
          />
        )}
        </div>
      </Flex>
    </Card>
  );
}
