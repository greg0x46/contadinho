import { DeleteOutlined, EditOutlined, PlusOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import { Alert, Button, Card, Drawer, Flex, Input, Popconfirm, Select, Space, Switch, Table, Tag, Typography } from "antd";
import { useEffect, useState } from "react";

import type { InvestmentAsset, InvestmentAssetWrite, InvestmentAssetClass, InvestmentAssetClassDefinition } from "../api/contracts";
import { useInvestmentAssets } from "../hooks/useInvestmentAssets";

const customType = "__custom";

type AssetDraft = {
  name: string;
  ticker: string;
  assetClass: InvestmentAssetClass | "";
  assetType: string;
  customType: string;
  currencyCode: string;
  quoteEnabled: boolean;
  quoteSymbol: string;
};

const emptyDraft: AssetDraft = {
  name: "", ticker: "", assetClass: "", assetType: "", customType: "",
  currencyCode: "BRL", quoteEnabled: false, quoteSymbol: "",
};

function draftFrom(asset: InvestmentAsset | null, classification: InvestmentAssetClassDefinition[]): AssetDraft {
  if (!asset) return emptyDraft;
  const known = classification.find((item) => item.asset_class === asset.asset_class)?.types.some((item) => item.name === asset.asset_type);
  return {
    name: asset.name,
    ticker: asset.ticker ?? "",
    assetClass: asset.asset_class,
    assetType: known ? asset.asset_type : customType,
    customType: known ? "" : asset.asset_type,
    currencyCode: asset.currency_code,
    quoteEnabled: asset.quote_source !== null,
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
      setDraft(draftFrom(editing, assets.classification));
      setFormError(null);
    }
  }, [open, editing, assets.classification]);

  const showCreate = () => {
    setEditing(null);
    setOpen(true);
  };

  const showEdit = (asset: InvestmentAsset) => {
    setEditing(asset);
    setOpen(true);
  };

  const classDefinition = assets.classification.find((item) => item.asset_class === draft.assetClass);
  const typeDefinition = classDefinition?.types.find((item) => item.name === draft.assetType);
  const legacyMarket = editing && draft.assetClass === editing.asset_class && draft.assetType === customType && draft.customType === editing.asset_type
    ? editing.quote_source : null;
  const quoteMarket = typeDefinition?.quote_market ?? legacyMarket;
  const quoteSource = draft.quoteEnabled ? quoteMarket : null;
  const classOptions = assets.classification.map((item) => ({ value: item.asset_class, label: item.label }));
  const typeOptions = [
    ...(classDefinition?.types.map((item) => ({ value: item.name, label: item.name })) ?? []),
    { value: customType, label: "Outro tipo" },
  ];

  const submit = async () => {
    const name = draft.name.trim();
    const assetType = (draft.assetType === customType ? draft.customType : draft.assetType).trim();
    const currencyCode = draft.currencyCode.trim().toUpperCase();
    if (name === "") {
      setFormError("Informe o nome do ativo.");
      return;
    }
    if (draft.assetClass === "") {
      setFormError("Escolha a classe do ativo.");
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
    // O código para cotação vem pré-preenchido com o que o sistema derivou do
    // código antigo. Se o código mudou e o campo não foi tocado, ele deixa de
    // valer: enviar vazio faz o sistema derivar de novo do código novo.
    const tickerChanged =
      editing !== null && (editing.ticker ?? "").trim().toLowerCase() !== draft.ticker.trim().toLowerCase();
    const symbolUntouched = editing !== null && draft.quoteSymbol.trim() === (editing.quote_symbol ?? "");
    const quoteSymbol = !quoteSource || (tickerChanged && symbolUntouched) ? "" : draft.quoteSymbol.trim();
    if (quoteSource && quoteSymbol === "" && draft.ticker.trim() === "") {
      setFormError("Informe o código do ativo, como PETR4 ou BTC, para buscar a cotação.");
      return;
    }
    const write: InvestmentAssetWrite = {
      name,
      ticker: draft.ticker.trim() || null,
      asset_type: assetType,
      asset_class: draft.assetClass,
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
    {
      title: "Classe", dataIndex: "asset_class",
      render: (value: InvestmentAssetClass) => assets.classification.find((item) => item.asset_class === value)?.label ?? value,
      filters: classOptions.map((item) => ({ text: item.label, value: item.value })),
      onFilter: (value, asset) => asset.asset_class === value,
    },
    { title: "Tipo", dataIndex: "asset_type" },
    { title: "Moeda", dataIndex: "currency_code", width: 100 },
    {
      title: "Ações",
      key: "actions",
      width: 210,
      render: (_, asset) => (
        <Space>
          <Button icon={<EditOutlined aria-hidden="true" />} onClick={() => showEdit(asset)} disabled={assets.classification.length === 0}>
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
        <Button type="primary" icon={<PlusOutlined aria-hidden="true" />} onClick={showCreate} disabled={assets.isLoading || assets.classification.length === 0}>
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
        dataSource={[...assets.assets].sort((left, right) => {
          const order = (value: InvestmentAssetClass) => assets.classification.findIndex((item) => item.asset_class === value);
          return order(left.asset_class) - order(right.asset_class) || left.name.localeCompare(right.name, "pt-BR");
        })}
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
            <label htmlFor="investment-asset-class">Classe</label>
            <Select
              id="investment-asset-class"
              value={draft.assetClass || undefined}
              placeholder="Selecione a classe financeira"
              options={classOptions}
              onChange={(value: InvestmentAssetClass) => setDraft((current) => ({ ...current,
                assetClass: value, assetType: "", customType: "", quoteEnabled: false, quoteSymbol: "" }))}
            />
          </div>
          <div className="filter-field">
            <label htmlFor="investment-asset-type">Tipo</label>
            <Select
              id="investment-asset-type"
              value={draft.assetType || undefined}
              disabled={!draft.assetClass}
              placeholder="Selecione o tipo de instrumento"
              options={typeOptions}
              showSearch
              optionFilterProp="label"
              onChange={(value: string) => setDraft((current) => ({ ...current, assetType: value,
                customType: "", quoteEnabled: Boolean(classDefinition?.types.find((item) => item.name === value)?.quote_market),
                quoteSymbol: "" }))}
            />
          </div>
          {draft.assetType === customType && (
            <div className="filter-field">
              <label htmlFor="investment-asset-custom-type">Descrição do tipo</label>
              <Input id="investment-asset-custom-type" value={draft.customType}
                onChange={(event) => setDraft((current) => ({ ...current, customType: event.target.value }))} />
            </div>
          )}
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
              {quoteSource ? "Código" : "Código (opcional)"}
            </label>
            <Input
              id="investment-asset-ticker"
              value={draft.ticker}
              placeholder="Ex.: PETR4"
              onChange={(event) => setDraft((current) => ({ ...current, ticker: event.target.value }))}
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
            <label htmlFor="investment-asset-quote-enabled">Cotação automática</label>
            <Switch id="investment-asset-quote-enabled" checked={draft.quoteEnabled && Boolean(quoteMarket)}
              disabled={!quoteMarket}
              onChange={(checked) => setDraft((current) => ({ ...current, quoteEnabled: checked }))} />
            <Typography.Text type="secondary">
              {quoteMarket ? "Atualiza o preço e busca o histórico do ativo em reais."
                : "Para este tipo, registre avaliações manualmente ou use os valores da instituição integrada."}
            </Typography.Text>
          </div>
          {quoteSource && (
            <details>
              <summary>Código alternativo para cotação</summary>
              <div className="filter-field">
                <label htmlFor="investment-asset-quote-symbol">Código para cotação (opcional)</label>
                <Input id="investment-asset-quote-symbol" value={draft.quoteSymbol}
                  placeholder={quoteSource === "crypto" ? "BTC" : "PETR4"}
                  onChange={(event) => setDraft((current) => ({ ...current, quoteSymbol: event.target.value }))} />
                <Typography.Text type="secondary">Em branco, usa o código do ativo. Preencha somente quando o código cotado for diferente.</Typography.Text>
              </div>
            </details>
          )}
        </Flex>
      </Drawer>
    </Card>
  );
}
