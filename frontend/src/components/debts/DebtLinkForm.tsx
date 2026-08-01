import { Alert, Button, Flex, Select } from "antd";
import { useState } from "react";

import type { EligibleTransaction } from "../../api/contracts";
import { formatOptionalDate } from "../../presentation/dates";
import { formatBRL } from "../../presentation/money";

function candidateLabel(candidate: EligibleTransaction): string {
  const date = formatOptionalDate(candidate.occurred_at);
  const description = candidate.description ?? "Sem descrição";
  const value = formatBRL(candidate.effective_money.value);
  return `${date} · ${description} · ${value}`;
}

export function DebtLinkForm({
  search,
  onSearchChange,
  candidates,
  isSearching,
  submitting,
  submitError,
  onSubmit,
}: {
  search: string;
  onSearchChange: (value: string) => void;
  candidates: EligibleTransaction[];
  isSearching: boolean;
  submitting: boolean;
  submitError: string | null;
  onSubmit: (transactionId: string) => void;
}) {
  const [selected, setSelected] = useState<string | null>(null);

  const submit = () => {
    if (selected === null) return;
    onSubmit(selected);
    setSelected(null);
  };

  return (
    <Flex vertical gap="small">
      {submitError && <Alert type="error" showIcon message={submitError} />}
      <Flex gap="small" wrap>
        <Select
          aria-label="Buscar transação para vincular"
          showSearch
          allowClear
          style={{ minWidth: 360 }}
          placeholder="Buscar por descrição"
          value={selected}
          searchValue={search}
          onSearch={onSearchChange}
          filterOption={false}
          loading={isSearching}
          notFoundContent={isSearching ? "Buscando…" : "Nenhuma transação elegível encontrada"}
          options={candidates.map((candidate) => ({
            value: candidate.id,
            label: candidateLabel(candidate),
          }))}
          onChange={(value: string | null) => setSelected(value)}
        />
        <Button type="primary" disabled={selected === null} loading={submitting} onClick={submit}>
          Vincular
        </Button>
      </Flex>
    </Flex>
  );
}
