import { PageContainer } from "@ant-design/pro-layout";
import { Alert, Button } from "antd";
import { useNavigate } from "react-router-dom";

import type { Investment } from "../api/contracts";
import { InvestmentList } from "../components/investments/InvestmentList";
import { InvestmentsSummary } from "../components/investments/InvestmentsSummary";
import { useInvestments } from "../hooks/useInvestments";

export function InvestmentsPage() {
  const investments = useInvestments();
  const navigate = useNavigate();

  const openDetail = (investment: Investment) => navigate(`/investimentos/${investment.id}`);

  return (
    <PageContainer
      title="Investimentos"
      subTitle="Acompanhe saldo e rendimento de cada investimento sincronizado"
      content="Os investimentos e suas movimentações são importados automaticamente da sua instituição financeira."
    >
      {investments.error && (
        <Alert
          type="error"
          showIcon
          message="Não foi possível carregar os investimentos"
          description={<Button onClick={() => investments.refetch()}>Tentar novamente</Button>}
          style={{ marginBottom: 16 }}
        />
      )}
      {!investments.isLoading && investments.investments.length > 0 && (
        <InvestmentsSummary investments={investments.investments} />
      )}
      <InvestmentList
        investments={investments.investments}
        isLoading={investments.isLoading}
        onOpen={openDetail}
      />
    </PageContainer>
  );
}
