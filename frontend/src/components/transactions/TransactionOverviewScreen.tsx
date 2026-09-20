import { LinkOutlined, RedoOutlined, SwapOutlined } from "@ant-design/icons";
import { Alert, Switch, Tag, Typography } from "antd";
import { useMemo } from "react";

import type { Category, TransactionInclusionState, TransactionItem } from "../../api/contracts";
import { useTransactionReconciliation } from "../../hooks/useTransactionReconciliation";
import { internalCategoryOriginLabel } from "../../presentation/categoryLabels";
import { formatDay } from "../../presentation/dates";
import { formatBRL } from "../../presentation/money";
import {
  allowsInvestmentLink,
  categoryChoices,
  detailValue,
  installmentLabel,
  investmentSplit,
  recurringCommitmentPrefill,
  remainderLabel,
  shortDateTime,
  suggestedCategory,
} from "../../presentation/transactionDetail";
import { classificationLabel } from "../../presentation/transactionStatus";
import { CategoryField } from "../categories/CategoryField";
import { PanelNavRow, PanelSection } from "../shared/PanelStack";
import type { TransactionScreen } from "./TransactionPanel";

/**
 * The root screen: who the transaction is and the decisions people take on
 * almost every one (category, whether it counts). Everything else is one
 * tap away behind a row, never open by default.
 */
export function TransactionOverviewScreen({
  item,
  categories,
  onCategory,
  categoryPending,
  onInclusion,
  inclusionPending,
  deleteError,
  navigate,
}: {
  item: TransactionItem;
  categories: Category[];
  onCategory?: (id: string, categoryId: string) => void;
  categoryPending: boolean;
  onInclusion?: (id: string, state: TransactionInclusionState) => void;
  inclusionPending: boolean;
  deleteError: string | null;
  navigate: (screen: TransactionScreen) => void;
}) {
  const ignored = item.inclusion.state === "ignored";
  const choices = useMemo(() => categoryChoices(categories, item), [categories, item]);
  const suggestion = suggestedCategory(item, choices);
  const split = investmentSplit(item);
  const prefill = recurringCommitmentPrefill(item);
  // The row only summarises; the reconcile screen owns the writes.
  const reconciliation = useTransactionReconciliation(item.id, !ignored);

  const where = item.card
    ? [item.card.number, installmentLabel(item.card)].filter(Boolean).join(" · ")
    : [item.account.name, item.account.institution].filter(Boolean).join(" · ");

  const reconciliationHint = ignored
    ? "Transações ignoradas não conciliam"
    : reconciliation.isLoading
      ? "Carregando…"
      : reconciliation.current
        ? `${reconciliation.current.commitment_name} · ${formatDay(reconciliation.current.occurrence_date)}`
        : reconciliation.options.length > 0
          ? `${reconciliation.options.length} ${reconciliation.options.length === 1 ? "ocorrência próxima" : "ocorrências próximas"}`
          : "Nenhuma recorrência por perto";

  const investmentHint = split
    ? `${formatBRL(split.transferred)} vinculados`
    : "Nenhum vínculo";

  return (
    <>
      <header className="transaction-identity">
        <div className="transaction-identity-kicker">
          <span>{classificationLabel[item.classification]}</span>
          {item.origin === "manual" && <Tag color="purple">Manual</Tag>}
        </div>
        <strong className="transaction-identity-description">
          {item.description ?? "Descrição não informada"}
        </strong>
        {where && <span className="transaction-identity-where">{where}</span>}
        <span className={`transaction-amount transaction-identity-amount amount-${item.classification}`}>
          {detailValue(item)}
        </span>
        <span className="transaction-identity-when">{shortDateTime(item.occurred_at)}</span>
        {split && (
          <span className="transaction-identity-split">
            {item.classification === "inflow" ? "resgate" : "aporte"} vinculado:{" "}
            {formatBRL(split.transferred)} · {remainderLabel(item, split.remaining)}
          </span>
        )}
      </header>

      <PanelSection title="Categoria">
        <CategoryField
          id="transaction-category"
          value={item.internal_category?.id ?? null}
          options={choices}
          suggested={suggestion}
          loading={categoryPending}
          disabled={!onCategory}
          onChange={(value) => onCategory?.(item.id, value)}
        />
        {item.internal_category && (
          <Typography.Text type="secondary" className="transaction-field-hint">
            {internalCategoryOriginLabel(item.internal_category)}
          </Typography.Text>
        )}
        {item.internal_category && !item.internal_category.is_active && (
          <Alert
            type="warning"
            showIcon
            message="Esta categoria foi desativada. Ela continua vigente para esta transação, mas não pode ser escolhida em outras."
          />
        )}
      </PanelSection>

      <PanelSection>
        <div className="transaction-toggle-row">
          <label htmlFor="transaction-considered">Considerar nos totais</label>
          <Switch
            id="transaction-considered"
            checked={!ignored}
            loading={inclusionPending}
            disabled={!onInclusion}
            onChange={(checked) => onInclusion?.(item.id, checked ? "considered" : "ignored")}
          />
        </div>
        {ignored && (
          <Typography.Text type="secondary" className="transaction-field-hint">
            Esta transação não será considerada nos relatórios e cálculos do período.
          </Typography.Text>
        )}
      </PanelSection>

      <PanelSection title="Ações" className="panel-section-rows">
        {prefill && (
          <PanelNavRow
            icon={<RedoOutlined />}
            label="Criar recorrência"
            onClick={() => navigate({ kind: "recurrence" })}
          />
        )}
        <PanelNavRow
          icon={<SwapOutlined />}
          label="Conciliar com recorrência"
          hint={reconciliationHint}
          disabled={ignored}
          onClick={() => navigate({ kind: "reconcile" })}
        />
        {allowsInvestmentLink(item) && (
          <PanelNavRow
            icon={<LinkOutlined />}
            label="Vincular a investimento"
            hint={investmentHint}
            onClick={() => navigate({ kind: "investment" })}
          />
        )}
      </PanelSection>

      <PanelSection className="panel-section-rows">
        <PanelNavRow label="Detalhes da transação" onClick={() => navigate({ kind: "details" })} />
        <PanelNavRow label="Informações técnicas" onClick={() => navigate({ kind: "technical" })} />
      </PanelSection>

      {item.origin === "manual" && split && (
        <Alert
          type="warning"
          showIcon
          message="Lançamento vinculado a investimento"
          description="Para editar ou excluir este lançamento, desfaça antes os vínculos em “Vincular a investimento”."
        />
      )}
      {deleteError && <Alert type="error" showIcon message={deleteError} />}
    </>
  );
}
