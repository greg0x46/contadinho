import { PlusOutlined } from "@ant-design/icons";
import { Alert, Button } from "antd";
import { useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

import type { Payable, PayableKind } from "../api/contracts";
import { DataCard, ListToolbar, Page, PageTabs, SearchField, SortSelect } from "../components/layout";
import { PayableForm } from "../components/payables/PayableForm";
import { PayableList } from "../components/payables/PayableList";
import { PayablesSummary } from "../components/payables/PayablesSummary";
import { usePayables } from "../hooks/usePayables";

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Não foi possível salvar a pendência.";
}

type PayableView = PayableKind | "all";
type PayableSort = "recent" | "name" | "remaining";

const viewOptions: { label: string; value: PayableView }[] = [
  { label: "Todas", value: "all" },
  { label: "Dívidas", value: "debt" },
  { label: "A receber", value: "receivable" },
];

const sortOptions: { label: string; value: PayableSort }[] = [
  { label: "Mais recentes", value: "recent" },
  { label: "Nome", value: "name" },
  { label: "Maior valor restante", value: "remaining" },
];

const comparators: Record<PayableSort, (left: Payable, right: Payable) => number> = {
  recent: (left, right) => right.created_at.localeCompare(left.created_at),
  name: (left, right) => left.name.localeCompare(right.name, "pt-BR"),
  remaining: (left, right) => Number(right.remaining_amount) - Number(left.remaining_amount),
};

/** Search and sort happen here: the list is small and already fully loaded. */
function arrange(payables: Payable[], search: string, sort: PayableSort): Payable[] {
  const needle = search.trim().toLocaleLowerCase("pt-BR");
  const matched = needle
    ? payables.filter((payable) => payable.name.toLocaleLowerCase("pt-BR").includes(needle))
    : payables;
  return [...matched].sort(comparators[sort]);
}

export function PayablesPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const kindParam = searchParams.get("kind");
  const filter: PayableView = kindParam === "debt" || kindParam === "receivable" ? kindParam : "all";
  const payables = usePayables(filter === "all" ? null : filter);
  const navigate = useNavigate();
  const [search, setSearch] = useState("");
  const [sort, setSort] = useState<PayableSort>("recent");
  const [formOpen, setFormOpen] = useState(false);
  const [formKind, setFormKind] = useState<PayableKind>("debt");
  const [editingPayable, setEditingPayable] = useState<Payable | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const setFilter = (value: PayableView) => {
    if (value === "all") {
      searchParams.delete("kind");
    } else {
      searchParams.set("kind", value);
    }
    setSearchParams(searchParams, { replace: true });
  };

  const openCreate = (kind: PayableKind) => {
    setFormKind(kind);
    setEditingPayable(null);
    setSaveError(null);
    setFormOpen(true);
  };

  const openEdit = (payable: Payable) => {
    setFormKind(payable.kind);
    setEditingPayable(payable);
    setSaveError(null);
    setFormOpen(true);
  };

  const closeForm = () => setFormOpen(false);

  const submit = async (write: {
    name: string;
    total_amount: number;
    initial_remaining_amount: number | null;
  }) => {
    setSaveError(null);
    try {
      if (editingPayable) {
        await payables.updatePayable({
          payableId: editingPayable.id,
          write: { name: write.name, total_amount: write.total_amount },
        });
      } else {
        await payables.createPayable({
          kind: formKind,
          name: write.name,
          total_amount: write.total_amount,
          initial_remaining_amount: write.initial_remaining_amount,
        });
      }
      setFormOpen(false);
    } catch (error) {
      setSaveError(errorMessage(error));
    }
  };

  const remove = async (payable: Payable) => {
    setActionError(null);
    try {
      await payables.deletePayable(payable.id);
    } catch (error) {
      setActionError(errorMessage(error));
    }
  };

  const openDetail = (payable: Payable) => navigate(`/pendencias/${payable.id}?kind=${payable.kind}`);

  const visible = arrange(payables.payables, search, sort);

  return (
    <Page
      title="Pendências"
      description="Acompanhe dívidas e contas a receber em um só lugar"
      actions={
        <>
          {filter !== "receivable" && (
            <Button
              type="primary"
              icon={<PlusOutlined aria-hidden="true" />}
              onClick={() => openCreate("debt")}
            >
              Nova dívida
            </Button>
          )}
          {filter !== "debt" && (
            <Button
              type={filter === "receivable" ? "primary" : "default"}
              icon={<PlusOutlined aria-hidden="true" />}
              onClick={() => openCreate("receivable")}
            >
              Nova conta a receber
            </Button>
          )}
        </>
      }
      tabs={<PageTabs label="Tipo de pendência" options={viewOptions} value={filter} onChange={setFilter} />}
    >
      {actionError && (
        <Alert
          type="error"
          showIcon
          closable
          onClose={() => setActionError(null)}
          message={actionError}
          style={{ marginBottom: 16 }}
        />
      )}
      {payables.error && (
        <Alert
          type="error"
          showIcon
          message="Não foi possível carregar as pendências"
          description={<Button onClick={() => payables.refetch()}>Tentar novamente</Button>}
          style={{ marginBottom: 16 }}
        />
      )}

      <ListToolbar
        label="Controles das pendências"
        start={
          <SearchField
            id="payables-search"
            label="Buscar pendências"
            placeholder="Buscar por nome…"
            value={search}
            onChange={setSearch}
          />
        }
        end={<SortSelect id="payables-sort" value={sort} options={sortOptions} onChange={setSort} />}
      />

      <DataCard
        flush
        summary={
          !payables.isLoading && filter !== "all" && payables.payables.length > 0 ? (
            <PayablesSummary kind={filter} payables={payables.payables} />
          ) : undefined
        }
      >
        <PayableList
          payables={visible}
          isLoading={payables.isLoading}
          onOpen={openDetail}
          onEdit={openEdit}
          onDelete={remove}
        />
      </DataCard>
      <PayableForm
        kind={formKind}
        open={formOpen}
        payable={editingPayable}
        submitting={payables.isSaving || payables.isUpdating}
        submitError={saveError}
        onSubmit={submit}
        onCancel={closeForm}
      />
    </Page>
  );
}
