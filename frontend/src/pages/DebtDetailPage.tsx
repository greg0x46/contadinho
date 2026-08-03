import { PageContainer } from "@ant-design/pro-layout";
import { Alert, Button, Flex } from "antd";
import { useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";

import { isUuid } from "../api/contracts";
import { LoadingState, UnavailableState } from "../components/AsyncState";
import { DebtForm } from "../components/debts/DebtForm";
import { DebtHeaderCard } from "../components/debts/DebtHeaderCard";
import { DebtTimeline } from "../components/debts/DebtTimeline";
import { useDebtDetail } from "../hooks/useDebtDetail";
import { useDebts } from "../hooks/useDebts";

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

function InvalidDebt() {
  return (
    <Alert
      type="error"
      showIcon
      message={<h1>Endereço de dívida inválido</h1>}
      description="O identificador informado não possui o formato esperado."
      action={<Link to="/dividas">Voltar para dívidas</Link>}
    />
  );
}

function ValidDebtDetail({ id }: { id: string }) {
  const debt = useDebtDetail(id);
  const debts = useDebts();
  const navigate = useNavigate();
  const [formOpen, setFormOpen] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const submitEdit = async (write: { name: string; total_amount: number }) => {
    setSaveError(null);
    try {
      await debts.updateDebt({ debtId: id, write: { name: write.name, total_amount: write.total_amount } });
      setFormOpen(false);
    } catch (error) {
      setSaveError(errorMessage(error, "Não foi possível salvar a dívida."));
    }
  };

  const submitDelete = async () => {
    setDeleteError(null);
    try {
      await debts.deleteDebt(id);
      navigate("/dividas");
    } catch (error) {
      setDeleteError(errorMessage(error, "Não foi possível excluir a dívida."));
    }
  };

  return (
    <PageContainer
      title="Detalhes da dívida"
      extra={<Link to="/dividas">Voltar para dívidas</Link>}
    >
      <Flex vertical gap="large">
        {debt.state.freshness === "loading" && <LoadingState>Carregando dívida…</LoadingState>}
        {debt.state.freshness === "not_found" && (
          <Alert
            type="error"
            showIcon
            message="Dívida não encontrada"
            description="Não existe uma dívida com este identificador."
            action={<Link to="/dividas">Voltar para dívidas</Link>}
          />
        )}
        {debt.state.freshness === "unavailable" && (
          <UnavailableState onRetry={debt.retry}>
            Não foi possível consultar esta dívida agora.
          </UnavailableState>
        )}
        {debt.state.snapshot !== null && (
          <>
            {debt.state.freshness === "stale" && (
              <Alert
                type="warning"
                showIcon
                message="As informações podem estar desatualizadas."
                action={
                  <Button loading={debt.state.retrying} onClick={debt.retry}>
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

            <DebtHeaderCard
              debt={debt.state.snapshot}
              onEdit={() => {
                setSaveError(null);
                setFormOpen(true);
              }}
              onDelete={submitDelete}
            />

            <DebtTimeline
              debtId={id}
              links={debt.state.snapshot.links}
              search={debt.search}
              onSearchChange={debt.setSearch}
              candidates={debt.eligibleTransactions}
              isSearching={debt.isSearching}
              isLinking={debt.isLinking}
              onLinkTransaction={debt.linkTransaction}
              onUnlinkTransaction={debt.unlinkTransaction}
            />
          </>
        )}
      </Flex>

      {debt.state.snapshot !== null && (
        <DebtForm
          open={formOpen}
          debt={debt.state.snapshot}
          submitting={debts.isUpdating}
          submitError={saveError}
          onSubmit={submitEdit}
          onCancel={() => setFormOpen(false)}
        />
      )}
    </PageContainer>
  );
}

export function DebtDetailPage() {
  const { id = "" } = useParams();
  return isUuid(id) ? <ValidDebtDetail id={id} /> : <InvalidDebt />;
}
