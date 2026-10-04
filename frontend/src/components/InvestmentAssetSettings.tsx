import { Alert, Button, Input, Skeleton, Table } from "antd";
import type { ColumnsType } from "antd/es/table";
import { useEffect, useState } from "react";

import type { InvestmentAsset, InvestmentAssetWrite } from "../api/contracts";
import { useInvestmentAssets } from "../hooks/useInvestmentAssets";
import { errorMessage } from "../presentation/errors";
import { FormDrawer } from "./forms/FormDrawer";
import { FormField } from "./forms/FormField";
import { RecordMenu } from "./shared/RecordMenu";
import { useConfirm } from "./shared/useConfirm";
import { DataCard, EmptyState, ResponsiveList } from "./layout";
import { useFeedback } from "./shared/useFeedback";

type AssetDraft = {
  name: string;
  ticker: string;
  assetType: string;
  currencyCode: string;
};

const emptyDraft: AssetDraft = { name: "", ticker: "", assetType: "", currencyCode: "BRL" };

function draftFrom(asset: InvestmentAsset | null): AssetDraft {
  if (!asset) return emptyDraft;
  return {
    name: asset.name,
    ticker: asset.ticker ?? "",
    assetType: asset.asset_type,
    currencyCode: asset.currency_code,
  };
}

/**
 * The asset registry: a list (rows on a phone, a flat table from `md` up)
 * and the form drawer. The "Novo ativo" action lives in the page's title row,
 * so it reaches this component as `creating` / `onCreatingClose`; editing
 * stays local because it starts from a row.
 */
export function InvestmentAssetSettings({
  creating,
  onCreatingClose,
}: {
  creating: boolean;
  onCreatingClose: () => void;
}) {
  const assets = useInvestmentAssets();
  const feedback = useFeedback();
  const confirm = useConfirm();
  const [editing, setEditing] = useState<InvestmentAsset | null>(null);
  const [draft, setDraft] = useState<AssetDraft>(emptyDraft);
  const [formError, setFormError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const open = creating || editing !== null;

  useEffect(() => {
    if (open) {
      setDraft(draftFrom(editing));
      setFormError(null);
    }
  }, [open, editing]);

  const close = () => {
    setEditing(null);
    onCreatingClose();
  };

  const submit = async () => {
    const name = draft.name.trim();
    const assetType = draft.assetType.trim();
    const currencyCode = draft.currencyCode.trim().toUpperCase();
    if (name === "") {
      setFormError("Informe o nome do ativo.");
      return;
    }
    if (assetType === "") {
      setFormError("Informe o tipo do ativo.");
      return;
    }
    if (!/^[A-Z]{3}$/.test(currencyCode)) {
      setFormError("Informe a moeda com três letras, como BRL ou USD.");
      return;
    }

    const write: InvestmentAssetWrite = {
      name,
      ticker: draft.ticker.trim() || null,
      asset_type: assetType,
      currency_code: currencyCode,
    };
    setFormError(null);
    try {
      if (editing) {
        await assets.updateAsset({ assetId: editing.id, write });
      } else {
        await assets.createAsset(write);
      }
      close();
      feedback.success("Salvo");
    } catch (error) {
      setFormError(errorMessage(error, "Não foi possível salvar o ativo."));
    }
  };

  const remove = async (asset: InvestmentAsset) => {
    setActionError(null);
    setDeletingId(asset.id);
    try {
      await assets.deleteAsset(asset.id);
      feedback.success("Excluído");
    } catch (error) {
      const message = errorMessage(error, "Não foi possível excluir o ativo.");
      setActionError(message);
      feedback.error(message);
    } finally {
      setDeletingId(null);
    }
  };

  const menu = (asset: InvestmentAsset) => (
    <RecordMenu
      label={`Ações de ${asset.name}`}
      items={[
        { key: "edit", label: "Editar", onClick: () => setEditing(asset) },
        {
          key: "delete",
          label: "Excluir",
          danger: true,
          disabled: deletingId !== null,
          onClick: () =>
            confirm({
              title: "Excluir ativo",
              description: "O ativo só pode ser excluído quando não estiver associado a nenhuma posição.",
              onConfirm: () => void remove(asset),
            }),
        },
      ]}
    />
  );

  const columns: ColumnsType<InvestmentAsset> = [
    {
      title: "Nome",
      dataIndex: "name",
      sorter: (left, right) => left.name.localeCompare(right.name, "pt-BR"),
    },
    {
      title: "Código",
      dataIndex: "ticker",
      render: (ticker: string | null) => ticker ?? <span className="investment-quiet">—</span>,
    },
    { title: "Tipo", dataIndex: "asset_type" },
    { title: "Moeda", dataIndex: "currency_code", width: 100 },
    {
      title: <span className="investment-visually-hidden">Ações</span>,
      key: "actions",
      width: 48,
      render: (_, asset) => <span onClick={(event) => event.stopPropagation()}>{menu(asset)}</span>,
    },
  ];

  return (
    <>
      {actionError && (
        <Alert
          type="error"
          showIcon
          closable
          message={actionError}
          onClose={() => setActionError(null)}
          style={{ marginBottom: 16 }}
        />
      )}
      {assets.error && (
        <Alert
          type="error"
          showIcon
          message="Não foi possível carregar os ativos"
          action={<Button onClick={() => assets.refetch()}>Tentar novamente</Button>}
          style={{ marginBottom: 16 }}
        />
      )}

      {/* A failed load shows only the retry: an empty list beside it would say "nothing yet". */}
      {!assets.error && (
        <DataCard flush className="settings-list-card">
          <ResponsiveList
            label="Ativos de investimento"
            items={assets.assets}
            getKey={(asset) => asset.id}
            isLoading={assets.isLoading}
            loading={
              <div role="status" aria-label="Carregando ativos">
                <Skeleton active paragraph={{ rows: 3 }} title={false} />
              </div>
            }
            // No action: the page's own "Novo ativo" is the way out.
            empty={
              <EmptyState
                title="Nenhum ativo cadastrado"
                hint="Cadastre o primeiro ativo para usá-lo nas posições de investimento."
              />
            }
            row={(asset) => ({
              title: asset.name,
              meta: [asset.ticker, asset.asset_type, asset.currency_code].filter(Boolean).join(" · "),
              trailing: menu(asset),
              ariaLabel: asset.name,
            })}
            wide={
              <Table<InvestmentAsset>
                aria-label="Ativos de investimento"
                className="investment-table"
                rowKey="id"
                columns={columns}
                dataSource={assets.assets}
                pagination={false}
                onRow={(asset) => ({ onClick: () => setEditing(asset), style: { cursor: "pointer" } })}
              />
            }
          />
        </DataCard>
      )}

      <FormDrawer
        title={editing ? "Editar ativo" : "Novo ativo"}
        open={open}
        onClose={close}
        onSubmit={() => void submit()}
        submitting={assets.isSaving}
        error={formError}
      >
        <FormField label="Nome" htmlFor="investment-asset-name">
          <Input
            id="investment-asset-name"
            value={draft.name}
            placeholder="Ex.: Tesouro Selic 2029"
            onChange={(event) => setDraft((current) => ({ ...current, name: event.target.value }))}
          />
        </FormField>
        <FormField label="Código (opcional)" htmlFor="investment-asset-ticker">
          <Input
            id="investment-asset-ticker"
            value={draft.ticker}
            placeholder="Ex.: PETR4"
            onChange={(event) => setDraft((current) => ({ ...current, ticker: event.target.value }))}
          />
        </FormField>
        <FormField label="Tipo" htmlFor="investment-asset-type">
          <Input
            id="investment-asset-type"
            value={draft.assetType}
            placeholder="Ex.: Ação, fundo ou renda fixa"
            onChange={(event) => setDraft((current) => ({ ...current, assetType: event.target.value }))}
          />
        </FormField>
        <FormField label="Moeda" htmlFor="investment-asset-currency">
          <Input
            id="investment-asset-currency"
            value={draft.currencyCode}
            maxLength={3}
            placeholder="BRL"
            onChange={(event) => setDraft((current) => ({ ...current, currencyCode: event.target.value.toUpperCase() }))}
          />
        </FormField>
      </FormDrawer>
    </>
  );
}
