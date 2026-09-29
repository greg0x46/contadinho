import { Link, useParams } from "react-router-dom";

import { isUuid } from "../api/contracts";
import { DetailPage, InvalidDetailPage } from "../components/layout";
import { InvestmentHeaderCard } from "../components/investments/InvestmentHeaderCard";
import { InvestmentTimeline } from "../components/investments/InvestmentTimeline";
import { useInvestmentDetail } from "../hooks/useInvestmentDetail";

const pageTitle = "Detalhes do investimento";
const backLink = <Link to="/investimentos">Voltar para investimentos</Link>;

function ValidInvestmentDetail({ id }: { id: string }) {
  const investment = useInvestmentDetail(id);

  return (
    <DetailPage
      title={pageTitle}
      back={backLink}
      state={investment.state}
      retry={investment.retry}
      loadingLabel="Carregando investimento…"
      notFoundMessage="Investimento não encontrado"
      notFoundDescription="Não existe um investimento com este identificador."
      unavailableMessage="Não foi possível consultar este investimento agora."
    >
      {(snapshot) => (
        <>
          <InvestmentHeaderCard investment={snapshot} transactions={investment.transactions} />

          <InvestmentTimeline
            transactions={investment.transactions}
            isLoading={investment.transactionsLoading}
            error={investment.transactionsError}
            onRetry={investment.retryTransactions}
          />
        </>
      )}
    </DetailPage>
  );
}

export function InvestmentDetailPage() {
  const { id = "" } = useParams();
  return isUuid(id) ? (
    <ValidInvestmentDetail id={id} />
  ) : (
    <InvalidDetailPage title={pageTitle} back={backLink} invalidTitle="Endereço de investimento inválido" />
  );
}
