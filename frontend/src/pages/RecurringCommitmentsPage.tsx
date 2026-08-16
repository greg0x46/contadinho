import { PlusOutlined } from "@ant-design/icons";
import { PageContainer } from "@ant-design/pro-layout";
import { Alert, Button } from "antd";
import { useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";

import type { RecurringCommitment, RecurringCommitmentWrite } from "../api/contracts";
import type { RecurringCommitmentDraft } from "../components/recurringCommitments/RecurringCommitmentForm";
import { RecurringCommitmentForm } from "../components/recurringCommitments/RecurringCommitmentForm";
import { RecurringCommitmentList } from "../components/recurringCommitments/RecurringCommitmentList";
import { useCategories } from "../hooks/useCategories";
import { useRecurringCommitments } from "../hooks/useRecurringCommitments";

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Não foi possível salvar o compromisso.";
}

export function RecurringCommitmentsPage() {
  const location = useLocation();
  const navigate = useNavigate();
  const commitments = useRecurringCommitments();
  const categories = useCategories();
  const [formOpen, setFormOpen] = useState(
    Boolean((location.state as { prefill?: Partial<RecurringCommitmentDraft> } | null)?.prefill),
  );
  const [editingCommitment, setEditingCommitment] = useState<RecurringCommitment | null>(null);
  const [initialDraft] = useState<Partial<RecurringCommitmentDraft> | null>(
    (location.state as { prefill?: Partial<RecurringCommitmentDraft> } | null)?.prefill ?? null,
  );
  const [togglingCommitmentId, setTogglingCommitmentId] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const openCreate = () => {
    setEditingCommitment(null);
    setSaveError(null);
    setFormOpen(true);
  };

  const openEdit = (commitment: RecurringCommitment) => {
    setEditingCommitment(commitment);
    setSaveError(null);
    setFormOpen(true);
  };

  const closeForm = () => {
    setFormOpen(false);
    if (location.state) navigate(".", { replace: true, state: null });
  };

  const submit = async (write: RecurringCommitmentWrite) => {
    setSaveError(null);
    try {
      if (editingCommitment) {
        await commitments.updateCommitment({ commitmentId: editingCommitment.id, write });
      } else {
        await commitments.createCommitment(write);
      }
      setFormOpen(false);
      if (location.state) navigate(".", { replace: true, state: null });
    } catch (error) {
      setSaveError(errorMessage(error));
    }
  };

  const toggle = async (commitment: RecurringCommitment, isActive: boolean) => {
    setActionError(null);
    setTogglingCommitmentId(commitment.id);
    try {
      await commitments.toggleCommitment({ commitmentId: commitment.id, isActive });
    } catch (error) {
      setActionError(errorMessage(error));
    } finally {
      setTogglingCommitmentId(null);
    }
  };

  const remove = async (commitment: RecurringCommitment) => {
    setActionError(null);
    try {
      await commitments.deleteCommitment(commitment.id);
    } catch (error) {
      setActionError(errorMessage(error));
    }
  };

  return (
    <PageContainer
      title="Recorrências"
      subTitle="Compromissos financeiros que se repetem"
      content="Cadastre salário, aluguel e assinaturas recorrentes para acompanhar esses compromissos mesmo antes de eles aparecerem como transação."
      extra={[
        <Button key="new-commitment" type="primary" icon={<PlusOutlined aria-hidden="true" />} onClick={openCreate}>
          Novo compromisso
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
      {commitments.error && (
        <Alert
          type="error"
          showIcon
          message="Não foi possível carregar os compromissos recorrentes"
          description={<Button onClick={() => commitments.refetch()}>Tentar novamente</Button>}
          style={{ marginBottom: 16 }}
        />
      )}
      <RecurringCommitmentList
        commitments={commitments.commitments}
        categories={categories.categories}
        isLoading={commitments.isLoading}
        togglingCommitmentId={togglingCommitmentId}
        onEdit={openEdit}
        onToggle={toggle}
        onDelete={remove}
      />
      <RecurringCommitmentForm
        open={formOpen}
        commitment={editingCommitment}
        initialDraft={editingCommitment ? null : initialDraft}
        categories={categories.categories}
        submitting={commitments.isSaving}
        submitError={saveError}
        onSubmit={submit}
        onCancel={closeForm}
      />
    </PageContainer>
  );
}
