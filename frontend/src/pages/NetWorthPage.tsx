import { PageContainer } from "@ant-design/pro-layout";
import { Alert, Empty } from "antd";

import { LoadingState, UnavailableState } from "../components/AsyncState";
import { NetWorthBreakdownCard } from "../components/netWorth/NetWorthBreakdownCard";
import { NetWorthChart } from "../components/netWorth/NetWorthChart";
import { useNetWorth } from "../hooks/useNetWorth";

export function NetWorthPage() {
  const netWorth = useNetWorth();
  // A single point has no evolution to plot — a line chart with one dot
  // reads as broken, not "just started". Wait for at least two days of
  // history before showing the chart.
  const hasSeries = netWorth.series.length > 1;
  const hasBackfilledPoints = netWorth.series.some((s) => s.is_backfilled);

  return (
    <PageContainer
      title="Patrimônio líquido"
      subTitle="Ativos menos passivos ao longo do tempo"
      content="Os dias mais recentes sem registro são reconstruídos automaticamente a partir do histórico de transações disponível — a série pode não cobrir todo o passado se as contas foram conectadas há pouco tempo."
    >
      {netWorth.isLoading && <LoadingState>Carregando patrimônio líquido…</LoadingState>}
      {netWorth.error && !netWorth.isLoading && (
        <UnavailableState onRetry={() => netWorth.refetch()}>
          Não foi possível carregar o patrimônio líquido
        </UnavailableState>
      )}

      {!netWorth.isLoading && !netWorth.error && netWorth.latest && (
        <>
          <NetWorthBreakdownCard snapshot={netWorth.latest} />
          {hasSeries ? (
            <>
              <NetWorthChart snapshots={netWorth.series} />
              {hasBackfilledPoints && (
                <Alert
                  type="info"
                  showIcon
                  message="Pontos reconstruídos não incluem investimentos"
                  description="Dias reconstruídos automaticamente (antes do primeiro acesso a esta página) não somam investimentos ao total: o rendimento de um investimento não tem uma data conhecida, então não há como saber quanto dele já existia em dias passados."
                  style={{ marginTop: 16 }}
                />
              )}
            </>
          ) : (
            <Empty description="O gráfico aparece a partir do segundo dia com dados registrados." />
          )}
        </>
      )}
    </PageContainer>
  );
}
