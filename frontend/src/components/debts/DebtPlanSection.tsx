import { Alert, Button, Flex, InputNumber, Typography } from "antd";
import { useState } from "react";

import type { ScenarioTransaction } from "../../api/contracts";
import { useDebtPlan } from "../../hooks/useDebtPlan";
import { DebtPlanTable } from "./DebtPlanTable";

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

export function DebtPlanSection({ debtId }: { debtId: string }) {
  const plan = useDebtPlan(debtId);
  const [months, setMonths] = useState<number | null>(6);
  const [actionError, setActionError] = useState<string | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);

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
        <DebtPlanTable
          installments={plan.plan.transactions}
          deletingId={deletingId}
          onDelete={deleteInstallment}
        />
      )}
    </Flex>
  );
}
