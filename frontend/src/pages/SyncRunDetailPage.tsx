import { Link, useParams } from "react-router-dom";

import { isUuid } from "../api/contracts";
import { DetailPage, InvalidDetailPage } from "../components/layout";
import { SyncFailureList } from "../components/SyncFailureList";
import { SyncRunOverview } from "../components/SyncRunOverview";
import { useSyncRun } from "../hooks/useSyncRun";

const pageTitle = "Detalhes da sincronização";
const backLink = <Link to="/configuracoes/open-banking">Voltar para sincronizações</Link>;

function ValidRunDetail({ id }: { id: string }) {
  const { state, retry } = useSyncRun(id);

  return (
    <DetailPage
      title={pageTitle}
      back={backLink}
      state={state}
      retry={retry}
      loadingLabel="Carregando sincronização…"
      notFoundMessage="Sincronização não encontrada"
      notFoundDescription="Não existe uma execução com este identificador."
      unavailableMessage="Não foi possível consultar esta sincronização agora."
    >
      {(snapshot) => (
        <>
          <SyncRunOverview run={snapshot} />
          <SyncFailureList failures={snapshot.failures} />
        </>
      )}
    </DetailPage>
  );
}

export function SyncRunDetailPage() {
  const { id = "" } = useParams();
  return isUuid(id) ? (
    <ValidRunDetail id={id} />
  ) : (
    <InvalidDetailPage title={pageTitle} back={backLink} invalidTitle="Endereço de sincronização inválido" />
  );
}
