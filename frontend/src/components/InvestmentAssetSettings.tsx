import { DeleteOutlined, EditOutlined, PlusOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import { Alert, Button, Card, Drawer, Flex, Input, Popconfirm, Select, Space, Table, Tag, Typography } from "antd";
import { useEffect, useState } from "react";

import type { InvestmentAsset, InvestmentAssetWrite } from "../api/contracts";
import { useInvestmentAssets } from "../hooks/useInvestmentAssets";

// A cotação automática é escolhida pelo mercado do instrumento, não por um
// provedor: o backend tenta os provedores configurados em ordem, com fallback.
const quoteSourceOptions = [
  { value: "", label: "Nenhuma" },
  { value: "b3", label: "B3 (ações, FIIs, ETFs, BDRs)" },
  { value: "crypto", label: "Criptomoeda" },
];

// Quem cadastra só informa o código do ativo e escolhe o mercado; o código
// para cotação é derivado do código do ativo. O campo abaixo só é necessário
// quando o código cotado no mercado difere do código do ativo.
const quoteSymbolPlaceholder: Record<string, string> = {
  b3: "PETR4",
  crypto: "BTC",
};

const quoteSymbolHint: Record<string, string> = {
  b3: "Em branco, usa o código do ativo. Informe só quando o código na B3 for diferente (ex.: PETR4; petr4, PETR4.SA ou BVMF:PETR4 também valem).",
  crypto: "Em branco, usa o código do ativo. Informe só quando o código da criptomoeda for diferente (ex.: BTC; btc, BTC-USD ou BTC/BRL também valem). Cotada em reais.",
};

type AssetDraft = {
  name: string;
  ticker: string;
  assetType: string;
  currencyCode: string;
  quoteSource: string;
  quoteSymbol: string;
};

const emptyDraft: AssetDraft = {
  name: "",
  ticker: "",
  assetType: "",
  currencyCode: "BRL",
  quoteSource: "",
  quoteSymbol: "",
};

function draftFrom(asset: InvestmentAsset | null): AssetDraft {
  if (!asset) return emptyDraft;
  return {
    name: asset.name,
    ticker: asset.ticker ?? "",
    assetType: asset.asset_type,
    currencyCode: asset.currency_code,
    quoteSource: asset.quote_source ?? "",
    quoteSymbol: asset.quote_symbol ?? "",
  };
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Não foi possível salvar o ativo.";
}

export function InvestmentAssetSettings() {
  const assets = useInvestmentAssets();
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<InvestmentAsset | null>(null);
  const [draft, setDraft] = useState<AssetDraft>(emptyDraft);
  const [formError, setFormError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setDraft(draftFrom(editing));
      setFormError(null);
    }
  }, [open, editing]);

  const showCreate = () => {
    setEditing(null);
    setOpen(true);
  };

  const showEdit = (asset: InvestmentAsset) => {
    setEditing(asset);
    setOpen(true);
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
    const quoteSource = draft.quoteSource.trim();
    // O código para cotação vem pré-preenchido com o que o sistema derivou do
    // código antigo. Se o código mudou e o campo não foi tocado, ele deixa de
    // valer: enviar vazio faz o sistema derivar de novo do código novo.
    const tickerChanged =
      editing !== null && (editing.ticker ?? "").trim().toLowerCase() !== draft.ticker.trim().toLowerCase();
    const symbolUntouched = editing !== null && draft.quoteSymbol.trim() === (editing.quote_symbol ?? "");
    const quoteSymbol = tickerChanged && symbolUntouched ? "" : draft.quoteSymbol.trim();
    if (quoteSource !== "" && quoteSymbol === "" && draft.ticker.trim() === "") {
      setFormError("Informe o código do ativo, como PETR4 ou BTC, para buscar a cotação.");
      return;
    }
    if (quoteSource === "" && quoteSymbol !== "") {
      setFormError("Escolha o mercado da cotação automática para o código informado.");
      return;
    }

    const write: InvestmentAssetWrite = {
      name,
      ticker: draft.ticker.trim() || null,
      asset_type: assetType,
      currency_code: currencyCode,
      quote_source: quoteSource || null,
      quote_symbol: quoteSymbol || null,
    };
    setFormError(null);
    try {
      if (editing) {
        await assets.updateAsset({ assetId: editing.id, write });
      } else {
        await assets.createAsset(write);
      }
      setOpen(false);
    } catch (error) {
      setFormError(errorMessage(error));
    }
  };

  const remove = async (asset: InvestmentAsset) => {
    setActionError(null);
    setDeletingId(asset.id);
    try {
      await assets.deleteAsset(asset.id);
    } catch (error) {
      setActionError(errorMessage(error));
    } finally {
      setDeletingId(null);
    }
  };

  const columns: ColumnsType<InvestmentAsset> = [
    {
      title: "Nome",
      dataIndex: "name",
      sorter: (left, right) => left.name.localeCompare(right.name, "pt-BR"),
    },
    {
      title: "Código",
      dataIndex: "ticker",
      render: (ticker: string | null) => ticker ? <Tag>{ticker}</Tag> : <Typography.Text type="secondary">—</Typography.Text>,
    },
    { title: "Tipo", dataIndex: "asset_type" },
    { title: "Moeda", dataIndex: "currency_code", width: 100 },
    {
      title: "Ações",
      key: "actions",
      width: 210,
      render: (_, asset) => (
        <Space>
          <Button icon={<EditOutlined aria-hidden="true" />} onClick={() => showEdit(asset)}>
            Editar
          </Button>
          <Popconfirm
            title="Excluir ativo"
            description="O ativo só pode ser excluído quando não estiver associado a nenhuma posição."
            okText="Excluir"
            cancelText="Cancelar"
            okButtonProps={{ danger: true }}
            onConfirm={() => remove(asset)}
          >
            <Button
              danger
              icon={<DeleteOutlined aria-hidden="true" />}
              loading={deletingId === asset.id}
              disabled={deletingId !== null && deletingId !== asset.id}
              aria-label={`Excluir ativo ${asset.name}`}
            >
              Excluir
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <Card
      title="Ativos de investimento"
      style={{ marginBottom: 24 }}
      extra={
        <Button type="primary" icon={<PlusOutlined aria-hidden="true" />} onClick={showCreate}>
          Novo ativo
        </Button>
      }
    >
      <Typography.Paragraph type="secondary">
        Cadastre os instrumentos usados nas posições de investimento. Alterações refletem em todas as contas que usam o ativo.
      </Typography.Paragraph>
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
      <Table<InvestmentAsset>
        aria-label="Ativos de investimento"
        className="investment-table"
        rowKey="id"
        columns={columns}
        dataSource={assets.assets}
        loading={assets.isLoading}
        pagination={false}
        scroll={{ x: "max-content" }}
        locale={{ emptyText: "Nenhum ativo cadastrado." }}
      />

      <Drawer
        title={editing ? "Editar ativo" : "Novo ativo"}
        open={open}
        onClose={() => setOpen(false)}
        width={420}
        destroyOnHidden
        footer={
          <Flex justify="end" gap="small">
            <Button onClick={() => setOpen(false)}>Cancelar</Button>
            <Button type="primary" loading={assets.isSaving} onClick={() => void submit()}>
              Salvar
            </Button>
          </Flex>
        }
      >
        <Flex vertical gap="middle">
          {formError && <Alert type="error" showIcon message={formError} />}
          <div className="filter-field">
            <label htmlFor="investment-asset-name">Nome</label>
            <Input
              id="investment-asset-name"
              value={draft.name}
              placeholder="Ex.: Tesouro Selic 2029"
              onChange={(event) => setDraft((current) => ({ ...current, name: event.target.value }))}
            />
          </div>
          <div className="filter-field">
            <label htmlFor="investment-asset-ticker">
              {draft.quoteSource === "" ? "Código (opcional)" : "Código"}
            </label>
            <Input
              id="investment-asset-ticker"
              value={draft.ticker}
              placeholder="Ex.: PETR4"
              onChange={(event) => setDraft((current) => ({ ...current, ticker: event.target.value }))}
            />
          </div>
          <div className="filter-field">
            <label htmlFor="investment-asset-type">Tipo</label>
            <Input
              id="investment-asset-type"
              value={draft.assetType}
              placeholder="Ex.: Ação, fundo ou renda fixa"
              onChange={(event) => setDraft((current) => ({ ...current, assetType: event.target.value }))}
            />
          </div>
          <div className="filter-field">
            <label htmlFor="investment-asset-currency">Moeda</label>
            <Input
              id="investment-asset-currency"
              value={draft.currencyCode}
              maxLength={3}
              placeholder="BRL"
              onChange={(event) => setDraft((current) => ({ ...current, currencyCode: event.target.value.toUpperCase() }))}
            />
          </div>
          <div className="filter-field">
            <label htmlFor="investment-asset-quote-source">Cotação automática</label>
            <Select
              id="investment-asset-quote-source"
              value={draft.quoteSource}
              options={quoteSourceOptions}
              onChange={(value) => setDraft((current) => ({ ...current, quoteSource: value }))}
            />
          </div>
          {draft.quoteSource !== "" && (
            <div className="filter-field">
              <label htmlFor="investment-asset-quote-symbol">Código para cotação (opcional)</label>
              <Input
                id="investment-asset-quote-symbol"
                value={draft.quoteSymbol}
                placeholder={quoteSymbolPlaceholder[draft.quoteSource]}
                onChange={(event) => setDraft((current) => ({ ...current, quoteSymbol: event.target.value }))}
              />
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                {quoteSymbolHint[draft.quoteSource]}
              </Typography.Text>
            </div>
          )}
        </Flex>
      </Drawer>
    </Card>
  );
}
