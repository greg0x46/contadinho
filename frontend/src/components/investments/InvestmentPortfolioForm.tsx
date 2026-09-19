import { Alert, Button, DatePicker, Drawer, Flex, Input, InputNumber } from "antd";
import dayjs, { type Dayjs } from "dayjs";
import { useEffect, useState } from "react";

import type { InvestmentPortfolio, InvestmentPortfolioWrite } from "../../api/contracts";

type Draft = {
  name: string;
  targetAmount: string | null;
  targetDate: Dayjs | null;
  notes: string;
};

function draftFrom(portfolio: InvestmentPortfolio | null): Draft {
  if (!portfolio) return { name: "", targetAmount: null, targetDate: null, notes: "" };
  return {
    name: portfolio.name,
    targetAmount: portfolio.target_amount,
    targetDate: portfolio.target_date ? dayjs(portfolio.target_date) : null,
    notes: portfolio.notes ?? "",
  };
}

export function InvestmentPortfolioForm({
  open,
  portfolio,
  submitting,
  submitError,
  onSubmit,
  onCancel,
}: {
  open: boolean;
  portfolio: InvestmentPortfolio | null;
  submitting: boolean;
  submitError: string | null;
  onSubmit: (write: InvestmentPortfolioWrite) => void;
  onCancel: () => void;
}) {
  const [draft, setDraft] = useState<Draft>(() => draftFrom(portfolio));
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setDraft(draftFrom(portfolio));
      setError(null);
    }
  }, [open, portfolio]);

  const submit = () => {
    if (draft.name.trim() === "") {
      setError("Informe o nome do objetivo.");
      return;
    }
    if (draft.targetAmount !== null && Number(draft.targetAmount) <= 0) {
      setError("A meta precisa ser maior que zero.");
      return;
    }
    setError(null);
    onSubmit({
      name: draft.name.trim(),
      target_amount: draft.targetAmount,
      target_date: draft.targetDate?.format("YYYY-MM-DD") ?? null,
      notes: draft.notes.trim() === "" ? null : draft.notes.trim(),
    });
  };

  return (
    <Drawer
      title={portfolio ? "Editar objetivo" : "Novo objetivo"}
      open={open}
      onClose={onCancel}
      width={460}
      destroyOnHidden
      footer={
        <Flex justify="end" gap="small">
          <Button onClick={onCancel}>Cancelar</Button>
          <Button type="primary" loading={submitting} onClick={submit}>
            Salvar
          </Button>
        </Flex>
      }
    >
      <Flex vertical gap="middle">
        {(error ?? submitError) && <Alert type="error" showIcon message={error ?? submitError} />}
        <div className="filter-field">
          <label htmlFor="investment-portfolio-name">Nome do objetivo</label>
          <Input
            id="investment-portfolio-name"
            value={draft.name}
            onChange={(event) => setDraft((current) => ({ ...current, name: event.target.value }))}
            placeholder="Ex.: Reserva de emergência"
            autoFocus
          />
        </div>
        <div className="filter-field">
          <label htmlFor="investment-portfolio-target">Meta em reais (opcional)</label>
          <InputNumber
            id="investment-portfolio-target"
            style={{ width: "100%" }}
            min="0.01"
            step="0.01"
            stringMode
            decimalSeparator=","
            value={draft.targetAmount}
            placeholder="0,00"
            onChange={(value) => setDraft((current) => ({ ...current, targetAmount: value === null ? null : String(value) }))}
          />
        </div>
        <div className="filter-field">
          <label htmlFor="investment-portfolio-date">Data-alvo (opcional)</label>
          <DatePicker
            id="investment-portfolio-date"
            style={{ width: "100%" }}
            format="DD/MM/YYYY"
            value={draft.targetDate}
            onChange={(value) => setDraft((current) => ({ ...current, targetDate: value }))}
          />
        </div>
        <div className="filter-field">
          <label htmlFor="investment-portfolio-notes">Observações (opcional)</label>
          <Input.TextArea
            id="investment-portfolio-notes"
            value={draft.notes}
            rows={4}
            onChange={(event) => setDraft((current) => ({ ...current, notes: event.target.value }))}
            placeholder="Para que serve este objetivo?"
          />
        </div>
      </Flex>
    </Drawer>
  );
}
