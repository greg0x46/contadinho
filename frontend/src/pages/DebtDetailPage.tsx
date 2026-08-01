import { PageContainer } from "@ant-design/pro-layout";
import { Alert, Button, Descriptions, Flex, Tag, Typography } from "antd";
import { useState } from "react";
import { Link, useParams } from "react-router-dom";

import { isUuid, type DebtLinkedTransaction } from "../api/contracts";
import { LoadingState, UnavailableState } from "../components/AsyncState";
import { DebtLinkForm } from "../components/debts/DebtLinkForm";
import { DebtLinkList } from "../components/debts/DebtLinkList";
import { useDebtDetail } from "../hooks/useDebtDetail";
import { debtStatusColor, debtStatusLabel } from "../presentation/debtLabels";
import { formatBRL } from "../presentation/money";

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
  const [linkError, setLinkError] = useState<string | null>(null);
  const [unlinkError, setUnlinkError] = useState<string | null>(null);
  const [unlinkingLinkId, setUnlinkingLinkId] = useState<string | null>(null);

  const submitLink = async (transactionId: string) => {
    setLinkError(null);
    try {
      await debt.linkTransaction(transactionId);
    } catch (error) {
      setLinkError(errorMessage(error, "Não foi possível vincular a transação."));
    }
  };

  const submitUnlink = async (link: DebtLinkedTransaction) => {
    setUnlinkError(null);
    setUnlinkingLinkId(link.id);
    try {
      await debt.unlinkTransaction(link.id);
    } catch (error) {
      setUnlinkError(errorMessage(error, "Não foi possível desvincular a transação."));
    } finally {
      setUnlinkingLinkId(null);
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
            <Descriptions bordered column={4} size="small">
              <Descriptions.Item label="Nome">{debt.state.snapshot.name}</Descriptions.Item>
              <Descriptions.Item label="Valor total">
                {formatBRL(debt.state.snapshot.total_amount)}
              </Descriptions.Item>
              <Descriptions.Item label="Valor pago">
                {formatBRL(debt.state.snapshot.paid_amount)}
              </Descriptions.Item>
              <Descriptions.Item label="Valor restante">
                {formatBRL(debt.state.snapshot.remaining_amount)}
              </Descriptions.Item>
              <Descriptions.Item label="Situação" span={4}>
                <Tag color={debtStatusColor[debt.state.snapshot.status]}>
                  {debtStatusLabel[debt.state.snapshot.status]}
                </Tag>
              </Descriptions.Item>
            </Descriptions>

            <section aria-labelledby="link-form-title">
              <Typography.Title id="link-form-title" level={2}>
                Vincular transação
              </Typography.Title>
              <DebtLinkForm
                search={debt.search}
                onSearchChange={debt.setSearch}
                candidates={debt.eligibleTransactions}
                isSearching={debt.isSearching}
                submitting={debt.isLinking}
                submitError={linkError}
                onSubmit={submitLink}
              />
            </section>

            <section aria-labelledby="links-title">
              <Typography.Title id="links-title" level={2}>
                Transações vinculadas
              </Typography.Title>
              {unlinkError && (
                <Alert
                  type="error"
                  showIcon
                  closable
                  onClose={() => setUnlinkError(null)}
                  message={unlinkError}
                  style={{ marginBottom: 16 }}
                />
              )}
              <DebtLinkList
                links={debt.state.snapshot.links}
                unlinkingLinkId={unlinkingLinkId}
                onUnlink={submitUnlink}
              />
            </section>
          </>
        )}
      </Flex>
    </PageContainer>
  );
}

export function DebtDetailPage() {
  const { id = "" } = useParams();
  return isUuid(id) ? <ValidDebtDetail id={id} /> : <InvalidDebt />;
}
