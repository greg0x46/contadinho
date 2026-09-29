import { ArrowLeftOutlined, InboxOutlined } from "@ant-design/icons";
import { Alert, Button, Card, Checkbox, Empty, Input, Radio, Select, Space, Spin, Table, Tag, Typography } from "antd";
import type { ColumnsType } from "antd/es/table";
import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";

import {
  confirmStatement, listImportAccounts, listStatementImports, previewStatement,
  type ImportAccount, type ImportHistoryItem, type ImportPreview, type ImportResult, type ImportRow,
} from "../api/statementImports";
import { Page } from "../components/layout";
import { formatDate, formatDateOnly } from "../presentation/dates";
import { formatMoney } from "../presentation/money";
import "../styles/statement-imports.css";

function message(error: unknown): string {
  return error instanceof Error ? error.message : "Não foi possível concluir a operação.";
}

function period(value: string | null): string {
  return value ? formatDateOnly(value) : "—";
}

const rowColumns: ColumnsType<ImportRow> = [
  { title: "Linha", dataIndex: "line_number", width: 70 },
  { title: "Data e hora", dataIndex: "occurred_at", render: (value: string | null) => value ? formatDate(value) : "—" },
  { title: "Movimentação", dataIndex: "description", render: (value: string) => value || "—" },
  { title: "Valor", key: "amount", align: "right", render: (_, row) => row.amount ? formatMoney(row.amount, row.currency ?? "BRL") : "—" },
  { title: "Saldo após", key: "balance", align: "right", render: (_, row) => row.balance ? formatMoney(row.balance, row.currency ?? "BRL") : "—" },
  { title: "Situação", dataIndex: "status", render: (value: ImportRow["status"]) => {
    const labels = { new: "Nova", duplicate: "Já importada", invalid: "Inválida" };
    const colors = { new: "green", duplicate: "default", invalid: "red" };
    return <Tag color={colors[value]}>{labels[value]}</Tag>;
  } },
  { title: "Revisão", key: "review", render: (_, row) => <div className="statement-row-notes">
    {row.errors.map((value, index) => <span key={`e-${index}`} className="statement-row-error">{value}</span>)}
    {row.warnings.map((value, index) => <span key={`w-${index}`}>{value}</span>)}
    {row.errors.length === 0 && row.warnings.length === 0 && "—"}
  </div> },
];

export function StatementImportPage() {
  const [accounts, setAccounts] = useState<ImportAccount[]>([]);
  const [history, setHistory] = useState<ImportHistoryItem[]>([]);
  const [historyError, setHistoryError] = useState("");
  const [accountsError, setAccountsError] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [targetKind, setTargetKind] = useState<"new" | "existing">("new");
  const [accountId, setAccountId] = useState<string>();
  const [newAccountName, setNewAccountName] = useState("");
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [result, setResult] = useState<ImportResult | null>(null);
  const [allowPartial, setAllowPartial] = useState(false);
  const [loading, setLoading] = useState(false);
  const previewRequest = useRef(0);
  const [error, setError] = useState("");

  const refreshLists = async () => {
    const [accountResult, historyResult] = await Promise.allSettled([listImportAccounts(), listStatementImports()]);
    if (accountResult.status === "fulfilled") { setAccounts(accountResult.value); setAccountsError(""); }
    else setAccountsError(message(accountResult.reason));
    if (historyResult.status === "fulfilled") { setHistory(historyResult.value); setHistoryError(""); }
    else setHistoryError(message(historyResult.reason));
  };

  useEffect(() => { void refreshLists(); }, []);

  async function loadPreview(selectedFile: File, selectedAccountId?: string) {
    const requestId = ++previewRequest.current;
    setLoading(true);
    setError("");
    setPreview(null);
    setResult(null);
    setAllowPartial(false);
    try {
      const next = await previewStatement(selectedFile, selectedAccountId);
      if (requestId === previewRequest.current) setPreview(next);
    } catch (cause) { if (requestId === previewRequest.current) setError(message(cause)); }
    finally { if (requestId === previewRequest.current) setLoading(false); }
  }

  function changeFile(selectedFile: File | null) {
    setFile(selectedFile);
    previewRequest.current++;
    setLoading(false);
    setPreview(null);
    setResult(null);
    setError("");
    if (selectedFile && (targetKind === "new" || accountId)) void loadPreview(selectedFile, targetKind === "existing" ? accountId : undefined);
  }

  function changeTarget(kind: "new" | "existing", id?: string) {
    previewRequest.current++;
    setLoading(false);
    setError("");
    setTargetKind(kind);
    setAccountId(id);
    setPreview(null);
    setResult(null);
    if (file && (kind === "new" || id)) void loadPreview(file, kind === "existing" ? id : undefined);
  }

  async function confirm() {
    if (!file || !preview) return;
    setLoading(true);
    setError("");
    try {
      const saved = await confirmStatement(file, preview,
        targetKind === "existing" ? { accountId } : { newAccountName: newAccountName.trim() }, allowPartial);
      setResult(saved);
      await refreshLists();
    } catch (cause) {
      setError(message(cause));
      if (cause instanceof Error && "kind" in cause && cause.kind === "conflict") setPreview(null);
    } finally { setLoading(false); }
  }

  const targetReady = targetKind === "new" ? newAccountName.trim().length > 0 : Boolean(accountId);
  const latest = preview?.rows.filter((row) => row.status !== "invalid" && row.occurred_at && row.balance)
    .sort((a, b) => b.occurred_at!.localeCompare(a.occurred_at!)) ?? [];
  const latestBalance = latest.length > 0 && !latest.some((row) => row.occurred_at === latest[0].occurred_at && row.balance !== latest[0].balance)
    ? latest[0].balance : null;
  const canConfirm = Boolean(file && preview && targetReady && !loading && !result &&
    (preview!.counts.invalid === 0 || allowPartial) && !(targetKind === "new" && preview!.counts.new === 0));

  return <Page title="Importar extrato" back={<Link to="/contas-e-cartoes"><ArrowLeftOutlined /> Contas e cartões</Link>}
    description="Revise as movimentações de um arquivo antes de adicioná-las à sua conta." className="statement-import-page">
    <div className="statement-import-stack">
      <Card title="1. Selecione o arquivo" className="statement-import-card">
        <label className="statement-file-picker">
          <InboxOutlined aria-hidden="true" />
          <span>{file ? file.name : "Selecionar arquivo CSV"}</span>
          <input type="file" accept=".csv,text/csv" aria-label="Arquivo CSV" onChange={(event) => changeFile(event.target.files?.[0] ?? null)} />
        </label>
        <Typography.Text type="secondary">O período é lido das movimentações, mesmo que o nome do arquivo indique outro mês.</Typography.Text>
      </Card>

      <Card title="2. Escolha a conta" className="statement-import-card">
        <Typography.Paragraph>Use uma conta de arquivo. Extratos enviados aqui ficam separados de conexões automáticas.</Typography.Paragraph>
        <Radio.Group value={targetKind} onChange={(event) => changeTarget(event.target.value)}>
          <Radio value="new">Criar conta de arquivo</Radio>
          <Radio value="existing" disabled={accounts.length === 0}>Usar conta existente</Radio>
        </Radio.Group>
        {accountsError && <Alert type="error" showIcon message="Não foi possível listar contas de arquivo" description={accountsError}
          action={<Button onClick={() => void refreshLists()}>Tentar novamente</Button>} />}
        <div className="statement-account-field">
          {targetKind === "new" ? <Input value={newAccountName} maxLength={100} placeholder="Nome da conta, por exemplo Flash"
            aria-label="Nome da nova conta" onChange={(event) => setNewAccountName(event.target.value)} /> :
            <Select value={accountId} placeholder="Selecione uma conta de arquivo" aria-label="Conta de arquivo" className="statement-account-select"
              options={accounts.map((account) => ({ value: account.id, label: `${account.name} (${account.currency_code})` }))}
              onChange={(value) => changeTarget("existing", value)} />}
        </div>
      </Card>

      {loading && <div role="status"><Spin /> Processando extrato…</div>}
      {error && <Alert type="error" showIcon message={error} action={file && !preview && !result ?
        <Button size="small" onClick={() => void loadPreview(file, targetKind === "existing" ? accountId : undefined)}>Gerar prévia novamente</Button> : undefined} />}
      {preview && <Card title="3. Revise a prévia" className="statement-import-card">
        <div className="statement-import-summary">
          <div><span>Formato</span><strong>{preview.format}</strong></div>
          <div><span>Período das movimentações</span><strong>{period(preview.period_start)} a {period(preview.period_end)}</strong></div>
          {latestBalance && <div><span>Saldo mais recente no arquivo</span><strong>{formatMoney(latestBalance, preview.currency)}</strong></div>}
          <div><span>Novas</span><strong>{preview.counts.new}</strong></div>
          <div><span>Já importadas</span><strong>{preview.counts.duplicate}</strong></div>
          <div><span>Inválidas</span><strong>{preview.counts.invalid}</strong></div>
        </div>
        {targetKind === "new" && preview.counts.new === 0 && <Alert type="info" showIcon message="Não há lançamentos válidos para criar esta conta." />}
        {preview.counts.new === 0 && preview.counts.duplicate > 0 &&
          <Alert type="info" showIcon message="Este extrato já foi importado nesta conta. Nenhum lançamento novo será criado." />}
        {preview.warnings.map((warning, index) => <Alert key={index} type="warning" showIcon message={warning} />)}
        {preview.counts.invalid > 0 && <Alert type="warning" showIcon
          message={`${preview.counts.invalid} linha(s) inválida(s). Corrija o arquivo ou escolha importar apenas as válidas.`} />}
        <Table columns={rowColumns} dataSource={preview.rows} rowKey="line_number" size="small" scroll={{ x: 850 }}
          pagination={{ pageSize: 20, showSizeChanger: false }} locale={{ emptyText: "Nenhuma linha para mostrar" }} />
        {preview.counts.invalid > 0 && <Checkbox checked={allowPartial} onChange={(event) => setAllowPartial(event.target.checked)}>
          Importar somente as linhas válidas; as inválidas serão ignoradas
        </Checkbox>}
        <div className="statement-confirm">
          <Button type="primary" loading={loading} disabled={!canConfirm} onClick={() => void confirm()}>
            {preview.counts.new === 0 ? "Registrar reenvio sem novos lançamentos" : `Importar ${preview.counts.new} lançamento(s)`}
          </Button>
          {!targetReady && <Typography.Text type="secondary">Informe a conta para confirmar.</Typography.Text>}
        </div>
      </Card>}
      {result && <Alert type="success" showIcon message={result.counts.new > 0 ? "Extrato importado" : "Extrato conferido: nenhum lançamento novo"}
        description={<Space direction="vertical"><span>{result.counts.new} novos, {result.counts.duplicate} já importados e {result.counts.invalid} inválidos.</span>
          <Link to={`/contas-e-cartoes/${result.account_id}`}>Ver conta</Link></Space>} />}

      <section aria-labelledby="statement-history-title">
        <Typography.Title id="statement-history-title" level={2}>Importações recentes</Typography.Title>
        {historyError && <Alert type="error" showIcon message="Não foi possível carregar o histórico" description={historyError}
          action={<Button onClick={() => void refreshLists()}>Tentar novamente</Button>} />}
        {!historyError && history.length === 0 && <Empty description="Nenhum extrato importado ainda" />}
        {history.length > 0 && <div className="statement-history-list">{history.map((item) =>
          <Card key={item.run_id} size="small">
            <strong>{item.account_name}</strong><span>{item.filename} · {formatDate(item.created_at)}</span>
            <span>{item.counts.new} novos · {item.counts.duplicate} já importados · {item.counts.invalid} inválidos</span>
            <Link to={`/contas-e-cartoes/${item.account_id}`}>Ver conta</Link>
          </Card>)}</div>}
      </section>
    </div>
  </Page>;
}
