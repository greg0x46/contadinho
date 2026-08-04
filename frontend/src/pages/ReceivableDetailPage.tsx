import { PageContainer } from "@ant-design/pro-layout";
import { Alert, Button, Flex } from "antd";
import { useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";

import { isUuid } from "../api/contracts";
import { LoadingState, UnavailableState } from "../components/AsyncState";
import { ReceivableForm } from "../components/receivables/ReceivableForm";
import { ReceivableHeaderCard } from "../components/receivables/ReceivableHeaderCard";
import { ReceivableTimeline } from "../components/receivables/ReceivableTimeline";
import { useReceivableDetail } from "../hooks/useReceivableDetail";
import { useReceivables } from "../hooks/useReceivables";

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

function InvalidReceivable() {
  return (
    <Alert
      type="error"
      showIcon
      message={<h1>Endereço de conta a receber inválido</h1>}
      description="O identificador informado não possui o formato esperado."
      action={<Link to="/contas-a-receber">Voltar para contas a receber</Link>}
    />
  );
}

function ValidReceivableDetail({ id }: { id: string }) {
  const receivable = useReceivableDetail(id);
  const receivables = useReceivables();
  const navigate = useNavigate();
  const [formOpen, setFormOpen] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const submitEdit = async (write: { name: string; total_amount: number }) => {
    setSaveError(null);
    try {
      await receivables.updateReceivable({
        receivableId: id,
        write: { name: write.name, total_amount: write.total_amount },
      });
      setFormOpen(false);
    } catch (error) {
      setSaveError(errorMessage(error, "Não foi possível salvar a conta a receber."));
    }
  };

  const submitDelete = async () => {
    setDeleteError(null);
    try {
      await receivables.deleteReceivable(id);
      navigate("/contas-a-receber");
    } catch (error) {
      setDeleteError(errorMessage(error, "Não foi possível excluir a conta a receber."));
    }
  };

  return (
    <PageContainer
      title="Detalhes da conta a receber"
      extra={<Link to="/contas-a-receber">Voltar para contas a receber</Link>}
    >
      <Flex vertical gap="large">
        {receivable.state.freshness === "loading" && (
          <LoadingState>Carregando conta a receber…</LoadingState>
        )}
        {receivable.state.freshness === "not_found" && (
          <Alert
            type="error"
            showIcon
            message="Conta a receber não encontrada"
            description="Não existe uma conta a receber com este identificador."
            action={<Link to="/contas-a-receber">Voltar para contas a receber</Link>}
          />
        )}
        {receivable.state.freshness === "unavailable" && (
          <UnavailableState onRetry={receivable.retry}>
            Não foi possível consultar esta conta a receber agora.
          </UnavailableState>
        )}
        {receivable.state.snapshot !== null && (
          <>
            {receivable.state.freshness === "stale" && (
              <Alert
                type="warning"
                showIcon
                message="As informações podem estar desatualizadas."
                action={
                  <Button loading={receivable.state.retrying} onClick={receivable.retry}>
                    Tentar novamente
                  </Button>
                }
              />
            )}
            {deleteError && (
              <Alert
                type="error"
                showIcon
                closable
                onClose={() => setDeleteError(null)}
                message={deleteError}
              />
            )}

            <ReceivableHeaderCard
              receivable={receivable.state.snapshot}
              onEdit={() => {
                setSaveError(null);
                setFormOpen(true);
              }}
              onDelete={submitDelete}
            />

            <ReceivableTimeline
              receivableId={id}
              links={receivable.state.snapshot.links}
              search={receivable.search}
              onSearchChange={receivable.setSearch}
              candidates={receivable.eligibleTransactions}
              isSearching={receivable.isSearching}
              isLinking={receivable.isLinking}
              onLinkTransaction={receivable.linkTransaction}
              onUnlinkTransaction={receivable.unlinkTransaction}
            />
          </>
        )}
      </Flex>

      {receivable.state.snapshot !== null && (
        <ReceivableForm
          open={formOpen}
          receivable={receivable.state.snapshot}
          submitting={receivables.isUpdating}
          submitError={saveError}
          onSubmit={submitEdit}
          onCancel={() => setFormOpen(false)}
        />
      )}
    </PageContainer>
  );
}

export function ReceivableDetailPage() {
  const { id = "" } = useParams();
  return isUuid(id) ? <ValidReceivableDetail id={id} /> : <InvalidReceivable />;
}
