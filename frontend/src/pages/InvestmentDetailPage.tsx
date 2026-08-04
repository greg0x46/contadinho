import { PageContainer } from "@ant-design/pro-layout";
import { Alert, Button, Flex } from "antd";
import { Link, useParams } from "react-router-dom";

import { isUuid } from "../api/contracts";
import { LoadingState, UnavailableState } from "../components/AsyncState";
import { InvestmentHeaderCard } from "../components/investments/InvestmentHeaderCard";
import { InvestmentTimeline } from "../components/investments/InvestmentTimeline";
import { useInvestmentDetail } from "../hooks/useInvestmentDetail";

function InvalidInvestment() {
  return (
    <Alert
      type="error"
      showIcon
      message={<h1>Endereço de investimento inválido</h1>}
      description="O identificador informado não possui o formato esperado."
      action={<Link to="/investimentos">Voltar para investimentos</Link>}
    />
  );
}

function ValidInvestmentDetail({ id }: { id: string }) {
  const investment = useInvestmentDetail(id);

  return (
    <PageContainer
      title="Detalhes do investimento"
      extra={<Link to="/investimentos">Voltar para investimentos</Link>}
    >
      <Flex vertical gap="large">
        {investment.state.freshness === "loading" && (
          <LoadingState>Carregando investimento…</LoadingState>
        )}
        {investment.state.freshness === "not_found" && (
          <Alert
            type="error"
            showIcon
            message="Investimento não encontrado"
            description="Não existe um investimento com este identificador."
            action={<Link to="/investimentos">Voltar para investimentos</Link>}
          />
        )}
        {investment.state.freshness === "unavailable" && (
          <UnavailableState onRetry={investment.retry}>
            Não foi possível consultar este investimento agora.
          </UnavailableState>
        )}
        {investment.state.snapshot !== null && (
          <>
            {investment.state.freshness === "stale" && (
              <Alert
                type="warning"
                showIcon
                message="As informações podem estar desatualizadas."
                action={
                  <Button loading={investment.state.retrying} onClick={investment.retry}>
                    Tentar novamente
                  </Button>
                }
              />
            )}

            <InvestmentHeaderCard
              investment={investment.state.snapshot}
              transactions={investment.transactions}
            />

            <InvestmentTimeline
              transactions={investment.transactions}
              isLoading={investment.transactionsLoading}
              error={investment.transactionsError}
              onRetry={investment.retryTransactions}
            />
          </>
        )}
      </Flex>
    </PageContainer>
  );
}

export function InvestmentDetailPage() {
  const { id = "" } = useParams();
  return isUuid(id) ? <ValidInvestmentDetail id={id} /> : <InvalidInvestment />;
}
