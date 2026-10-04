import { Input } from "antd";
import { useEffect, useState } from "react";

import type { Payable, PayableKind } from "../../api/contracts";
import { payableVocabulary } from "../../presentation/payableLabels";
import { FormDrawer } from "../forms/FormDrawer";
import { FormField } from "../forms/FormField";
import { MoneyInput } from "../forms/MoneyInput";

type Draft = {
  name: string;
  totalAmount: number | null;
  initialRemainingAmount: number | null;
};

const blankDraft: Draft = { name: "", totalAmount: null, initialRemainingAmount: null };

function draftFrom(payable: Payable | null): Draft {
  return payable
    ? { name: payable.name, totalAmount: Number(payable.total_amount), initialRemainingAmount: null }
    : blankDraft;
}

export function PayableForm({
  kind,
  open,
  payable,
  submitting,
  submitError,
  onSubmit,
  onCancel,
}: {
  kind: PayableKind;
  open: boolean;
  payable: Payable | null;
  submitting: boolean;
  submitError: string | null;
  onSubmit: (write: { name: string; total_amount: number; initial_remaining_amount: number | null }) => void;
  onCancel: () => void;
}) {
  const vocab = payableVocabulary[kind];
  const [draftState, setDraftState] = useState<Draft>(() => draftFrom(payable));
  const [error, setError] = useState<string | null>(null);
  const isEditing = payable !== null;

  useEffect(() => {
    if (open) {
      setDraftState(draftFrom(payable));
      setError(null);
    }
  }, [open, payable]);

  const submit = () => {
    if (draftState.name.trim() === "") {
      setError(vocab.nameRequiredError);
      return;
    }
    if (draftState.totalAmount === null || draftState.totalAmount <= 0) {
      setError("Informe um valor total maior que zero.");
      return;
    }
    if (!isEditing && draftState.initialRemainingAmount !== null) {
      if (draftState.initialRemainingAmount < 0) {
        setError("O valor restante inicial não pode ser negativo.");
        return;
      }
      if (draftState.initialRemainingAmount > draftState.totalAmount) {
        setError("O valor restante inicial não pode ser maior que o valor total.");
        return;
      }
    }
    setError(null);
    onSubmit({
      name: draftState.name.trim(),
      total_amount: draftState.totalAmount,
      initial_remaining_amount: isEditing ? null : draftState.initialRemainingAmount,
    });
  };

  return (
    <FormDrawer
      title={isEditing ? vocab.editTitle : vocab.newTitle}
      open={open}
      onClose={onCancel}
      onSubmit={submit}
      submitting={submitting}
      error={error ?? submitError}
    >
      <FormField label="Nome" htmlFor="payable-name">
        <Input
          id="payable-name"
          value={draftState.name}
          onChange={(event) => setDraftState((current) => ({ ...current, name: event.target.value }))}
          placeholder={vocab.nameFieldPlaceholder}
        />
      </FormField>

      <FormField label="Valor total" htmlFor="payable-total-amount">
        <MoneyInput
          id="payable-total-amount"
          min={0.01}
          value={draftState.totalAmount}
          onChange={(value) => setDraftState((current) => ({ ...current, totalAmount: value }))}
        />
      </FormField>

      {!isEditing && (
        <FormField
          label="Valor restante inicial (opcional)"
          htmlFor="payable-initial-remaining-amount"
          hint="Informe só se parte já foi quitada; sem isso, vale o valor total."
        >
          <MoneyInput
            id="payable-initial-remaining-amount"
            min={0}
            value={draftState.initialRemainingAmount}
            onChange={(value) => setDraftState((current) => ({ ...current, initialRemainingAmount: value }))}
            placeholder="Padrão: igual ao valor total"
          />
        </FormField>
      )}
    </FormDrawer>
  );
}
