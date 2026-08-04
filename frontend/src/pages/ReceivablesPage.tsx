import { PlusOutlined } from "@ant-design/icons";
import { PageContainer } from "@ant-design/pro-layout";
import { Alert, Button } from "antd";
import { useState } from "react";
import { useNavigate } from "react-router-dom";

import type { Receivable } from "../api/contracts";
import { ReceivableForm } from "../components/receivables/ReceivableForm";
import { ReceivableList } from "../components/receivables/ReceivableList";
import { ReceivablesSummary } from "../components/receivables/ReceivablesSummary";
import { useReceivables } from "../hooks/useReceivables";

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Não foi possível salvar a conta a receber.";
}

export function ReceivablesPage() {
  const receivables = useReceivables();
  const navigate = useNavigate();
  const [formOpen, setFormOpen] = useState(false);
  const [editingReceivable, setEditingReceivable] = useState<Receivable | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const openCreate = () => {
    setEditingReceivable(null);
    setSaveError(null);
    setFormOpen(true);
  };

  const openEdit = (receivable: Receivable) => {
    setEditingReceivable(receivable);
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
      if (editingReceivable) {
        await receivables.updateReceivable({
          receivableId: editingReceivable.id,
          write: { name: write.name, total_amount: write.total_amount },
        });
      } else {
        await receivables.createReceivable({
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

  const remove = async (receivable: Receivable) => {
    setActionError(null);
    try {
      await receivables.deleteReceivable(receivable.id);
    } catch (error) {
      setActionError(errorMessage(error));
    }
  };

  const openDetail = (receivable: Receivable) => navigate(`/contas-a-receber/${receivable.id}`);

  return (
    <PageContainer
      title="Contas a receber"
      subTitle="Acompanhe quanto falta receber em cada conta"
      content="Cadastre contas a receber e vincule transações já importadas para acompanhar o valor restante automaticamente."
      extra={[
        <Button
          key="new-receivable"
          type="primary"
          icon={<PlusOutlined aria-hidden="true" />}
          onClick={openCreate}
        >
          Nova conta a receber
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
      {receivables.error && (
        <Alert
          type="error"
          showIcon
          message="Não foi possível carregar as contas a receber"
          description={<Button onClick={() => receivables.refetch()}>Tentar novamente</Button>}
          style={{ marginBottom: 16 }}
        />
      )}
      {!receivables.isLoading && receivables.receivables.length > 0 && (
        <ReceivablesSummary receivables={receivables.receivables} />
      )}
      <ReceivableList
        receivables={receivables.receivables}
        isLoading={receivables.isLoading}
        onOpen={openDetail}
        onEdit={openEdit}
        onDelete={remove}
      />
      <ReceivableForm
        open={formOpen}
        receivable={editingReceivable}
        submitting={receivables.isSaving || receivables.isUpdating}
        submitError={saveError}
        onSubmit={submit}
        onCancel={closeForm}
      />
    </PageContainer>
  );
}
