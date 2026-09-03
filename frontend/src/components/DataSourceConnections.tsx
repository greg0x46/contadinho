import { Alert, Button, Card, Flex, Input, List, Space, Tag, Typography } from "antd";
import { SyncOutlined } from "@ant-design/icons";
import { useState } from "react";

import type { DataSource } from "../api/contracts";
import { ApiError } from "../api/problems";
import { LoadingState, UnavailableState } from "./AsyncState";
import type { useCreateSyncRun } from "../hooks/useCreateSyncRun";
import { useDataSources } from "../hooks/useDataSources";

function errorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    return error.problem?.detail ?? error.problem?.title ?? fallback;
  }
  return fallback;
}

/**
 * The registry of banks the Contadinho syncs. Each connection is one Pluggy
 * item, created in the Pluggy dashboard and pasted here — adding a second bank
 * is a row, not a reinstall.
 */
export function DataSourceConnections({ sync }: { sync: ReturnType<typeof useCreateSyncRun> }) {
  const { state, retry, create, update } = useDataSources();
  const [itemId, setItemId] = useState("");
  const [label, setLabel] = useState("");

  const submitNew = () => {
    if (itemId.trim() === "") {
      return;
    }
    create.mutate(
      { external_item_id: itemId.trim(), label: label.trim() === "" ? null : label.trim() },
      {
        onSuccess: () => {
          setItemId("");
          setLabel("");
        },
      },
    );
  };

  const renderConnection = (source: DataSource) => (
    <List.Item
      actions={[
        <Button
          key="sync"
          type="link"
          icon={<SyncOutlined aria-hidden />}
          disabled={!source.is_active || sync.state.kind === "submitting"}
          onClick={() => void sync.submit(source.id)}
        >
          Sincronizar
        </Button>,
        <Button
          key="active"
          type="link"
          loading={update.isPending && update.variables?.id === source.id}
          onClick={() => update.mutate({ id: source.id, changes: { is_active: !source.is_active } })}
        >
          {source.is_active ? "Desativar" : "Reativar"}
        </Button>,
      ]}
    >
      <List.Item.Meta
        title={
          <Space>
            <Typography.Text
              editable={{
                onChange: (next) => update.mutate({ id: source.id, changes: { label: next } }),
                tooltip: "Renomear conexão",
                triggerType: ["icon", "text"],
              }}
            >
              {source.name}
            </Typography.Text>
            {source.is_active ? (
              <Tag color="green">Ativa</Tag>
            ) : (
              <Tag>Desativada</Tag>
            )}
          </Space>
        }
        description={
          <Typography.Text type="secondary" copyable>
            {source.external_item_id}
          </Typography.Text>
        }
      />
    </List.Item>
  );

  return (
    <Card>
      <Flex vertical gap="middle">
        <div>
          <Typography.Title id="connections-title" level={2}>
            Conexões
          </Typography.Title>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
            Cada conexão é um Item do Pluggy. Crie o item no painel do Pluggy e informe o Item ID
            aqui para que ele passe a ser sincronizado junto com os demais.
          </Typography.Paragraph>
        </div>

        {state.kind === "loading" && <LoadingState>Carregando conexões…</LoadingState>}
        {state.kind === "unavailable" && (
          <UnavailableState onRetry={retry}>
            As conexões estão temporariamente indisponíveis. Isso não significa que não existam.
          </UnavailableState>
        )}
        {state.kind === "empty" && (
          <Alert
            type="info"
            showIcon
            message="Nenhuma conexão cadastrada."
            description="Informe abaixo o Item ID de um Item criado no painel do Pluggy para começar a sincronizar."
          />
        )}
        {state.kind === "ready" && (
          <List
            aria-label="Conexões"
            dataSource={state.sources}
            rowKey="id"
            renderItem={renderConnection}
          />
        )}

        {update.isError && (
          <Alert
            type="error"
            showIcon
            message={errorMessage(update.error, "Não foi possível atualizar a conexão.")}
          />
        )}
        {create.isError && (
          <Alert
            type="error"
            showIcon
            message={errorMessage(create.error, "Não foi possível adicionar a conexão.")}
          />
        )}

        <Flex gap="small" wrap align="flex-end">
          <div className="filter-field" style={{ flex: "2 1 260px" }}>
            <label htmlFor="connection-item-id">Item ID (Pluggy)</label>
            <Input
              id="connection-item-id"
              value={itemId}
              onChange={(event) => setItemId(event.target.value)}
              onPressEnter={submitNew}
            />
          </div>
          <div className="filter-field" style={{ flex: "1 1 180px" }}>
            <label htmlFor="connection-label">Apelido (opcional)</label>
            <Input
              id="connection-label"
              value={label}
              placeholder="Ex.: Conta pessoal"
              onChange={(event) => setLabel(event.target.value)}
              onPressEnter={submitNew}
            />
          </div>
          <Button
            type="primary"
            loading={create.isPending}
            disabled={itemId.trim() === ""}
            onClick={submitNew}
          >
            Adicionar conexão
          </Button>
        </Flex>
      </Flex>
    </Card>
  );
}
