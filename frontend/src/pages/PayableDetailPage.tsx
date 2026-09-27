import { Alert } from "antd";
import { useState } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router-dom";

import { isUuid, type PayableKind } from "../api/contracts";
import { DetailPage, InvalidDetailPage } from "../components/layout";
import { PayableForm } from "../components/payables/PayableForm";
import { PayableHeaderCard } from "../components/payables/PayableHeaderCard";
import { PayableTimeline } from "../components/payables/PayableTimeline";
import { usePayableDetail } from "../hooks/usePayableDetail";
import { usePayables } from "../hooks/usePayables";

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

const pageTitle = "Detalhes";
const backLink = <Link to="/pendencias">Voltar para pendências</Link>;

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
    <DetailPage
      title={pageTitle}
      back={backLink}
      state={payable.state}
      retry={payable.retry}
      loadingLabel="Carregando…"
      notFoundMessage="Não encontrada"
      notFoundDescription="Não existe uma dívida ou conta a receber com este identificador."
      unavailableMessage="Não foi possível consultar esta pendência agora."
    >
      {(snapshot) => (
        <>
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
            payable={snapshot}
            onEdit={() => {
              setSaveError(null);
              setFormOpen(true);
            }}
            onDelete={submitDelete}
          />

          <PayableTimeline
            payableId={id}
            kind={kind}
            links={snapshot.links}
            search={payable.search}
            onSearchChange={payable.setSearch}
            candidates={payable.eligibleTransactions}
            isSearching={payable.isSearching}
            isLinking={payable.isLinking}
            onLinkTransaction={payable.linkTransaction}
            onUnlinkTransaction={payable.unlinkTransaction}
          />

          <PayableForm
            kind={kind}
            open={formOpen}
            payable={snapshot}
            submitting={payables.isUpdating}
            submitError={saveError}
            onSubmit={submitEdit}
            onCancel={() => setFormOpen(false)}
          />
        </>
      )}
    </DetailPage>
  );
}

export function PayableDetailPage() {
  const { id = "" } = useParams();
  const [searchParams] = useSearchParams();
  const kindParam = searchParams.get("kind");
  const kind: PayableKind = kindParam === "receivable" ? "receivable" : "debt";
  return isUuid(id) ? (
    <ValidPayableDetail id={id} kind={kind} />
  ) : (
    <InvalidDetailPage title={pageTitle} back={backLink} invalidTitle="Endereço inválido" />
  );
}
