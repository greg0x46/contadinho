import { Alert, Button, Flex, InputNumber, Modal, Select, Typography } from "antd";
import { useState } from "react";

import type { DebtLinkedTransaction, ScenarioTransaction } from "../../api/contracts";
import { useDebtPlan } from "../../hooks/useDebtPlan";
import { formatOptionalDate } from "../../presentation/dates";
import { formatBRL } from "../../presentation/money";
import { DebtPlanTable } from "./DebtPlanTable";

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

function linkLabel(link: DebtLinkedTransaction): string {
  const date = formatOptionalDate(link.occurred_at);
  const description = link.description ?? "Sem descrição";
  return `${date} · ${description} · ${formatBRL(link.current_amount)}`;
}

// AccumulatedDeviationBanner surfaces roadmap task 6's Σ amount − Σ
// realizedTotal across every due installment: positive means less was
// actually allocated than planned so far ("atrasado"), negative means more
// ("adiantado"). Only due installments feed this number (computed
// server-side), so an unpaid future installment never counts against it.
function AccumulatedDeviationBanner({ value }: { value: string }) {
  const isAhead = value.startsWith("-");
  const isOnTrack = !isAhead && Number(value) === 0;
  const magnitude = formatBRL(isAhead ? value.slice(1) : value);

  if (isOnTrack) {
    return (
      <Alert type="success" showIcon message="Em dia com o plano de pagamento até hoje." />
    );
  }
  return (
    <Alert
      type={isAhead ? "success" : "warning"}
      showIcon
      message={
        isAhead
          ? `Adiantado ${magnitude} em relação ao plano até hoje.`
          : `Atrasado ${magnitude} em relação ao plano até hoje.`
      }
    />
  );
}

export function DebtPlanSection({
  debtId,
  links,
}: {
  debtId: string;
  links: DebtLinkedTransaction[];
}) {
  const plan = useDebtPlan(debtId);
  const [months, setMonths] = useState<number | null>(6);
  const [actionError, setActionError] = useState<string | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [allocatingInstallment, setAllocatingInstallment] = useState<ScenarioTransaction | null>(null);
  const [selectedLinkId, setSelectedLinkId] = useState<string | null>(null);
  const [allocatedAmount, setAllocatedAmount] = useState<number | null>(null);

  const runAction = async (action: () => Promise<unknown>, fallback: string) => {
    setActionError(null);
    try {
      await action();
    } catch (error) {
      setActionError(errorMessage(error, fallback));
    }
  };

  const createPlan = () => runAction(() => plan.createPlan("Plano de pagamento"), "Não foi possível criar o plano.");

  const generate = () => {
    if (months === null || months < 1) return;
    return runAction(
      () => plan.generateInstallments({ months }),
      "Não foi possível gerar as parcelas.",
    );
  };

  const deleteInstallment = async (installment: ScenarioTransaction) => {
    setDeletingId(installment.id);
    try {
      await plan.deleteInstallment(installment.id);
    } catch (error) {
      setActionError(errorMessage(error, "Não foi possível excluir a parcela."));
    } finally {
      setDeletingId(null);
    }
  };

  const openAllocation = (installment: ScenarioTransaction) => {
    setAllocatingInstallment(installment);
    setSelectedLinkId(null);
    setAllocatedAmount(Number(installment.amount));
  };

  const closeAllocation = () => {
    setAllocatingInstallment(null);
    setSelectedLinkId(null);
    setAllocatedAmount(null);
  };

  const confirmAllocation = async () => {
    if (allocatingInstallment === null || selectedLinkId === null || allocatedAmount === null || allocatedAmount <= 0) {
      return;
    }
    try {
      await plan.allocateRealization({
        transactionId: allocatingInstallment.id,
        write: { debt_link_id: selectedLinkId, allocated_amount: allocatedAmount },
      });
      closeAllocation();
    } catch (error) {
      setActionError(errorMessage(error, "Não foi possível alocar a transação."));
    }
  };

  if (plan.isLoading) {
    return <Typography.Text type="secondary">Carregando plano de pagamento…</Typography.Text>;
  }

  return (
    <Flex vertical gap="middle">
      {actionError && (
        <Alert type="error" showIcon closable onClose={() => setActionError(null)} message={actionError} />
      )}

      {plan.plan === null ? (
        <Flex gap="small" align="center">
          <Typography.Text type="secondary">
            Esta dívida ainda não tem um plano de pagamento.
          </Typography.Text>
          <Button type="primary" loading={plan.isCreating} onClick={createPlan}>
            Criar plano de pagamento
          </Button>
        </Flex>
      ) : plan.plan.transactions.length === 0 ? (
        <Flex gap="small" align="center" wrap>
          <Typography.Text>Gerar parcelas mensais a partir do valor restante em</Typography.Text>
          <InputNumber
            aria-label="Número de meses"
            min={1}
            max={120}
            value={months}
            onChange={(value) => setMonths(value)}
          />
          <Typography.Text>meses</Typography.Text>
          <Button type="primary" loading={plan.isGenerating} onClick={generate}>
            Gerar parcelas
          </Button>
        </Flex>
      ) : (
        <>
          <AccumulatedDeviationBanner value={plan.plan.accumulated_deviation} />
          <DebtPlanTable
            installments={plan.plan.transactions}
            deletingId={deletingId}
            onDelete={deleteInstallment}
            onAllocate={openAllocation}
          />
        </>
      )}

      <Modal
        title="Alocar transação vinculada"
        open={allocatingInstallment !== null}
        onCancel={closeAllocation}
        onOk={confirmAllocation}
        okText="Alocar"
        okButtonProps={{
          disabled: selectedLinkId === null || allocatedAmount === null || allocatedAmount <= 0,
          loading: plan.isAllocating,
        }}
      >
        <Flex vertical gap="small">
          <Typography.Text>
            Qual transação vinculada quita a parcela «{allocatingInstallment?.description}»?
          </Typography.Text>
          <Select
            aria-label="Transação vinculada"
            placeholder="Selecione uma transação vinculada"
            value={selectedLinkId}
            onChange={(value: string | null) => setSelectedLinkId(value)}
            options={links.map((link) => ({ value: link.id, label: linkLabel(link) }))}
            notFoundContent="Nenhuma transação vinculada a esta dívida ainda."
          />
          <InputNumber
            aria-label="Valor alocado"
            addonBefore="R$"
            min={0.01}
            step={0.01}
            style={{ width: "100%" }}
            value={allocatedAmount}
            onChange={(value) => setAllocatedAmount(value)}
          />
        </Flex>
      </Modal>
    </Flex>
  );
}
