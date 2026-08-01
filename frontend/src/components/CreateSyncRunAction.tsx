import { Alert, Button, Card, Flex, Typography } from "antd";
import { SyncOutlined } from "@ant-design/icons";
import { Link, useNavigate } from "react-router-dom";

import { useCreateSyncRun } from "../hooks/useCreateSyncRun";

export function CreateSyncRunAction() {
  const navigate = useNavigate();
  const { state, submit, reset } = useCreateSyncRun((id) =>
    navigate(`/open-banking/sync-runs/${id}`),
  );

  return (
    <Card>
      <Flex vertical gap="middle">
        <div>
          <Typography.Title id="create-title" level={2}>
            Atualizar dados
          </Typography.Title>
          <Typography.Paragraph>
            Busque agora os dados mais recentes disponíveis na sua conexão.
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
        {state.kind === "conflict" && (
          <Alert
            type="warning"
            showIcon
            message="Já existe uma sincronização em andamento."
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
