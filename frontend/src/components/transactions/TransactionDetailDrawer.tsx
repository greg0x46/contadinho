import { Alert, Button, Collapse, Descriptions, Drawer, Grid, Popconfirm, Select, Tag } from "antd";
import { useMemo } from "react";
import { useNavigate } from "react-router-dom";

import type { Category, TransactionInclusionState, TransactionItem } from "../../api/contracts";
import type { RecurringCommitmentDraft } from "../recurringCommitments/RecurringCommitmentForm";
import {
  categoryKindLabel,
  internalCategoryOriginLabel,
  renderCategoryIcon,
} from "../../presentation/categoryLabels";
import { formatMoney, formatSignedBRL, moneySourceLabel } from "../../presentation/money";
import { movementTypeLabel } from "../../presentation/transactionLabels";
import {
  classificationColor,
  classificationLabel,
  exclusionReasonLabel,
  inclusionOriginLabel,
} from "../../presentation/transactionStatus";
import { TransactionReconciliationSection } from "./TransactionReconciliationSection";

function detailValue(item: TransactionItem): string {
  if (!item.effective_money || item.effective_money.currency_code !== "BRL") {
    return "Valor indisponível em reais";
  }
  return formatSignedBRL(item.effective_money.value, item.classification);
}

function dateTime(value: string | null): string {
  if (!value) return "Data não informada";
  return new Intl.DateTimeFormat("pt-BR", {
    dateStyle: "long",
    timeStyle: "short",
  }).format(new Date(value));
}

function recurringCommitmentPrefill(item: TransactionItem): Partial<RecurringCommitmentDraft> | null {
  if (!item.effective_money || item.effective_money.currency_code !== "BRL" || !item.occurred_at) {
    return null;
  }
  const amount = Math.abs(Number(item.effective_money.value));
  if (!Number.isFinite(amount) || amount <= 0) return null;
  return {
    name: item.description ?? "",
    kind: item.classification === "inflow" ? "income" : "expense",
    amount,
    categoryId: item.internal_category?.id ?? null,
    dayOfMonth: new Date(item.occurred_at).getDate(),
  };
}

function installmentLabel(card: TransactionItem["card"]): string | null {
  if (!card || card.installment_number == null || card.total_installments == null) return null;
  return `Parcela ${card.installment_number}/${card.total_installments}`;
}

function categoryOptions(categories: Category[], item: TransactionItem | null) {
  const byId = new Map(categories.filter((category) => category.is_active).map((category) => [category.id, category]));
  if (item?.internal_category) {
    byId.set(item.internal_category.id, {
      id: item.internal_category.id,
      name: item.internal_category.name,
      kind: item.internal_category.kind,
      is_active: item.internal_category.is_active,
      icon: item.internal_category.icon,
      color: item.internal_category.color,
      created_at: "",
      updated_at: "",
    });
  }
  const kindOrder = ["expense", "income", "transfer"] as const;
  return [...byId.values()]
    .sort((a, b) => {
      if (a.kind !== b.kind) return kindOrder.indexOf(a.kind) - kindOrder.indexOf(b.kind);
      return a.name.localeCompare(b.name, "pt-BR");
    })
    .map((category) => ({
      value: category.id,
      label: `${categoryKindLabel[category.kind]}: ${category.name}${category.is_active ? "" : " (inativa)"}`,
      icon: category.icon,
      color: category.color,
    }));
}

export function TransactionDetailDrawer({
  item,
  categories = [],
  onClose,
  onInclusion,
  inclusionPending = false,
  onCategory,
  categoryPending = false,
  onEditManual,
  onDeleteManual,
  deleteManualPending = false,
  deleteManualError = null,
}: {
  item: TransactionItem | null;
  categories?: Category[];
  onClose: () => void;
  onInclusion?: (id: string, state: TransactionInclusionState) => void;
  inclusionPending?: boolean;
  onCategory?: (id: string, categoryId: string) => void;
  categoryPending?: boolean;
  onEditManual?: (item: TransactionItem) => void;
  onDeleteManual?: (id: string) => void;
  deleteManualPending?: boolean;
  deleteManualError?: string | null;
}) {
  const screens = Grid.useBreakpoint();
  const reason = item?.totals_eligibility.reason;
  const options = useMemo(() => categoryOptions(categories, item), [categories, item]);
  const navigate = useNavigate();
  const prefill = item ? recurringCommitmentPrefill(item) : null;

  return (
    <Drawer
      title="Detalhes da transação"
      open={item !== null}
      onClose={onClose}
      placement={screens.md ? "right" : "bottom"}
      width={480}
      height="82vh"
      destroyOnHidden
    >
      {item && (
        <div className="transaction-detail">
          <div className="transaction-detail-heading">
            <span>{item.description ?? "Descrição não informada"}</span>
            <strong className={`transaction-amount transaction-detail-amount amount-${item.classification}`}>
              {detailValue(item)}
            </strong>
            <div className="transaction-detail-tags">
              <Tag color={classificationColor[item.classification]}>
                {classificationLabel[item.classification]}
              </Tag>
              <Tag color={item.inclusion.state === "ignored" ? "default" : "success"}>
                {item.inclusion.state === "ignored" ? "Ignorada" : "Considerada"}
              </Tag>
              {item.origin === "manual" && <Tag color="purple">Manual</Tag>}
            </div>
          </div>

          <Descriptions title="Quando & onde" column={1} size="small" colon={false}>
            <Descriptions.Item label="Data e hora">
              {dateTime(item.occurred_at)}
            </Descriptions.Item>
            <Descriptions.Item label="Conta">
              {item.account.name ?? "Conta não informada"}
            </Descriptions.Item>
            <Descriptions.Item label="Instituição">
              {item.account.institution ?? "Instituição não informada"}
            </Descriptions.Item>
            {item.card && (
              <Descriptions.Item label="Cartão">
                {item.card.number}
                {installmentLabel(item.card) && (
                  <Tag className="transaction-card-installment" color="blue">
                    {installmentLabel(item.card)}
                  </Tag>
                )}
              </Descriptions.Item>
            )}
          </Descriptions>

          <Descriptions title="Categoria" column={1} size="small" colon={false}>
            <Descriptions.Item label="Categoria">
              <Select
                aria-label="Categoria"
                style={{ minWidth: 240 }}
                value={item.internal_category?.id}
                placeholder="Sem categoria"
                loading={categoryPending}
                disabled={!onCategory}
                showSearch
                optionFilterProp="label"
                options={options}
                optionRender={(option) => (
                  <span>
                    <span style={{ color: option.data.color }} aria-hidden="true">
                      {renderCategoryIcon(option.data.icon)}
                    </span>{" "}
                    {option.label}
                  </span>
                )}
                onChange={(value: string) => onCategory?.(item.id, value)}
              />
              {item.internal_category && (
                <div className="transaction-category-origin">
                  <span style={{ color: item.internal_category.color }} aria-hidden="true">
                    {renderCategoryIcon(item.internal_category.icon)}
                  </span>{" "}
                  {internalCategoryOriginLabel(item.internal_category)}
                </div>
              )}
              {item.internal_category && !item.internal_category.is_active && (
                <Alert
                  type="warning"
                  showIcon
                  style={{ marginTop: 8 }}
                  message="Esta categoria foi desativada. Ela continua vigente para esta transação, mas não pode ser escolhida em outras."
                />
              )}
            </Descriptions.Item>
            <Descriptions.Item label="Categoria sugerida (Pluggy)">
              {item.source_category ?? "Não informada"}
              <div className="transaction-category-origin">
                Sugestão do provedor — não afeta filtros ou totais.
              </div>
            </Descriptions.Item>
          </Descriptions>

          <Descriptions title="Decisão" column={1} size="small" colon={false}>
            <Descriptions.Item label="Decisão">
              {item.inclusion.state === "ignored" ? "Ignorada" : "Considerada"}
            </Descriptions.Item>
            {item.inclusion.changed_at && (
              <>
                <Descriptions.Item label="Origem da decisão">
                  {inclusionOriginLabel(item.inclusion)}
                </Descriptions.Item>
                <Descriptions.Item label="Última mudança">
                  {dateTime(item.inclusion.changed_at)}
                </Descriptions.Item>
              </>
            )}
          </Descriptions>
          <div className="transaction-detail-decision-actions">
            <Button
              loading={inclusionPending}
              disabled={!onInclusion}
              onClick={() =>
                onInclusion?.(
                  item.id,
                  item.inclusion.state === "ignored" ? "considered" : "ignored",
                )
              }
            >
              {item.inclusion.state === "ignored" ? "Restaurar" : "Ignorar"}
            </Button>
            {prefill && (
              <Button
                onClick={() =>
                  navigate("/recorrencias", { state: { prefill } })
                }
              >
                Criar recorrência a partir desta transação
              </Button>
            )}
            {item.origin === "manual" && (
              <>
                <Button disabled={!onEditManual} onClick={() => onEditManual?.(item)}>
                  Editar
                </Button>
                <Popconfirm
                  title="Excluir lançamento manual"
                  description="Esta ação não pode ser desfeita."
                  okText="Excluir"
                  cancelText="Cancelar"
                  okButtonProps={{ danger: true, loading: deleteManualPending }}
                  onConfirm={() => onDeleteManual?.(item.id)}
                  disabled={!onDeleteManual}
                >
                  <Button danger disabled={!onDeleteManual} loading={deleteManualPending}>
                    Excluir
                  </Button>
                </Popconfirm>
              </>
            )}
          </div>
          {deleteManualError && (
            <Alert type="error" showIcon message={deleteManualError} />
          )}

          <TransactionReconciliationSection
            transactionId={item.id}
            ignored={item.inclusion.state === "ignored"}
          />

          <Collapse
            ghost
            items={[
              {
                key: "technical",
                label: "Informações técnicas",
                children: (
                  <Descriptions column={1} size="small" colon={false}>
                    <Descriptions.Item label="Identificador externo">
                      {item.external_id}
                    </Descriptions.Item>
                    <Descriptions.Item label="Tipo original">
                      {movementTypeLabel(item.movement_type)}
                    </Descriptions.Item>
                    <Descriptions.Item label="Valor original">
                      {item.amount !== null && item.currency_code
                        ? formatMoney(item.amount, item.currency_code)
                        : "Não informado"}
                    </Descriptions.Item>
                    <Descriptions.Item label="Moeda original">
                      {item.currency_code ?? "Não informada"}
                    </Descriptions.Item>
                    <Descriptions.Item label="Status original da integração">
                      {item.provider_status ?? "Não informado"}
                    </Descriptions.Item>
                    {item.effective_money && (
                      <Descriptions.Item label="Origem do valor exibido">
                        {moneySourceLabel(item.effective_money.source)}
                      </Descriptions.Item>
                    )}
                    <Descriptions.Item label="Totalização">
                      {item.totals_eligibility.included
                        ? "Incluída nos totais"
                        : reason
                          ? exclusionReasonLabel[reason]
                          : "Não incluída"}
                    </Descriptions.Item>
                  </Descriptions>
                ),
              },
            ]}
          />
        </div>
      )}
    </Drawer>
  );
}
