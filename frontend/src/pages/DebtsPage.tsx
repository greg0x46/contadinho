import { PlusOutlined } from "@ant-design/icons";
import { PageContainer } from "@ant-design/pro-layout";
import { Alert, Button } from "antd";
import { useState } from "react";
import { useNavigate } from "react-router-dom";

import type { Debt } from "../api/contracts";
import { DebtForm } from "../components/debts/DebtForm";
import { DebtList } from "../components/debts/DebtList";
import { DebtsSummary } from "../components/debts/DebtsSummary";
import { useDebts } from "../hooks/useDebts";

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Não foi possível salvar a dívida.";
}

export function DebtsPage() {
  const debts = useDebts();
  const navigate = useNavigate();
  const [formOpen, setFormOpen] = useState(false);
  const [editingDebt, setEditingDebt] = useState<Debt | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const openCreate = () => {
    setEditingDebt(null);
    setSaveError(null);
    setFormOpen(true);
  };

  const openEdit = (debt: Debt) => {
    setEditingDebt(debt);
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
      if (editingDebt) {
        await debts.updateDebt({
          debtId: editingDebt.id,
          write: { name: write.name, total_amount: write.total_amount },
        });
      } else {
        await debts.createDebt({
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

  const remove = async (debt: Debt) => {
    setActionError(null);
    try {
      await debts.deleteDebt(debt.id);
    } catch (error) {
      setActionError(errorMessage(error));
    }
  };

  const openDetail = (debt: Debt) => navigate(`/dividas/${debt.id}`);

  return (
    <PageContainer
      title="Dívidas"
      subTitle="Acompanhe quanto falta pagar em cada dívida"
      content="Cadastre dívidas e vincule transações já importadas para acompanhar o valor restante automaticamente."
      extra={[
        <Button
          key="new-debt"
          type="primary"
          icon={<PlusOutlined aria-hidden="true" />}
          onClick={openCreate}
        >
          Nova dívida
        </Button>,
      ]}
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
      {debts.error && (
        <Alert
          type="error"
          showIcon
          message="Não foi possível carregar as dívidas"
          description={<Button onClick={() => debts.refetch()}>Tentar novamente</Button>}
          style={{ marginBottom: 16 }}
        />
      )}
      {!debts.isLoading && debts.debts.length > 0 && <DebtsSummary debts={debts.debts} />}
      <DebtList
        debts={debts.debts}
        isLoading={debts.isLoading}
        onOpen={openDetail}
        onEdit={openEdit}
        onDelete={remove}
      />
      <DebtForm
        open={formOpen}
        debt={editingDebt}
        submitting={debts.isSaving || debts.isUpdating}
        submitError={saveError}
        onSubmit={submit}
        onCancel={closeForm}
      />
    </PageContainer>
  );
}
