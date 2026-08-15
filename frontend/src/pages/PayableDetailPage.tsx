import { PageContainer } from "@ant-design/pro-layout";
import { Alert, Button, Flex } from "antd";
import { useState } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router-dom";

import { isUuid, type PayableKind } from "../api/contracts";
import { LoadingState, UnavailableState } from "../components/AsyncState";
import { PayableForm } from "../components/payables/PayableForm";
import { PayableHeaderCard } from "../components/payables/PayableHeaderCard";
import { PayableTimeline } from "../components/payables/PayableTimeline";
import { usePayableDetail } from "../hooks/usePayableDetail";
import { usePayables } from "../hooks/usePayables";

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

function InvalidPayable() {
  return (
    <Alert
      type="error"
      showIcon
      message={<h1>Endereço inválido</h1>}
      description="O identificador informado não possui o formato esperado."
      action={<Link to="/pendencias">Voltar para pendências</Link>}
    />
  );
}

function ValidPayableDetail({ id, kind }: { id: string; kind: PayableKind }) {
  const payable = usePayableDetail(id, kind);
  const payables = usePayables(kind);
  const navigate = useNavigate();
  const [formOpen, setFormOpen] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const submitEdit = async (write: { name: string; total_amount: number }) => {
    setSaveError(null);
    try {
      await payables.updatePayable({
        payableId: id,
        write: { name: write.name, total_amount: write.total_amount },
      });
      setFormOpen(false);
    } catch (error) {
      setSaveError(errorMessage(error, "Não foi possível salvar a pendência."));
    }
  };

  const submitDelete = async () => {
    setDeleteError(null);
    try {
      await payables.deletePayable(id);
      navigate("/pendencias");
    } catch (error) {
      setDeleteError(errorMessage(error, "Não foi possível excluir a pendência."));
    }
  };

  return (
    <PageContainer title="Detalhes" extra={<Link to="/pendencias">Voltar para pendências</Link>}>
      <Flex vertical gap="large">
        {payable.state.freshness === "loading" && <LoadingState>Carregando…</LoadingState>}
        {payable.state.freshness === "not_found" && (
          <Alert
            type="error"
            showIcon
            message="Não encontrada"
            description="Não existe uma dívida ou conta a receber com este identificador."
            action={<Link to="/pendencias">Voltar para pendências</Link>}
          />
        )}
        {payable.state.freshness === "unavailable" && (
          <UnavailableState onRetry={payable.retry}>
            Não foi possível consultar esta pendência agora.
          </UnavailableState>
        )}
        {payable.state.snapshot !== null && (
          <>
            {payable.state.freshness === "stale" && (
              <Alert
                type="warning"
                showIcon
                message="As informações podem estar desatualizadas."
                action={
                  <Button loading={payable.state.retrying} onClick={payable.retry}>
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

            <PayableHeaderCard
              payable={payable.state.snapshot}
              onEdit={() => {
                setSaveError(null);
                setFormOpen(true);
              }}
              onDelete={submitDelete}
            />

            <PayableTimeline
              payableId={id}
              kind={kind}
              links={payable.state.snapshot.links}
              search={payable.search}
              onSearchChange={payable.setSearch}
              candidates={payable.eligibleTransactions}
              isSearching={payable.isSearching}
              isLinking={payable.isLinking}
              onLinkTransaction={payable.linkTransaction}
              onUnlinkTransaction={payable.unlinkTransaction}
            />
          </>
        )}
      </Flex>

      {payable.state.snapshot !== null && (
        <PayableForm
          kind={kind}
          open={formOpen}
          payable={payable.state.snapshot}
          submitting={payables.isUpdating}
          submitError={saveError}
          onSubmit={submitEdit}
          onCancel={() => setFormOpen(false)}
        />
      )}
    </PageContainer>
  );
}

export function PayableDetailPage() {
  const { id = "" } = useParams();
  const [searchParams] = useSearchParams();
  const kindParam = searchParams.get("kind");
  const kind: PayableKind = kindParam === "receivable" ? "receivable" : "debt";
  return isUuid(id) ? <ValidPayableDetail id={id} kind={kind} /> : <InvalidPayable />;
}
