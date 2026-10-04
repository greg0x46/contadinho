import { Alert, Button, Radio, Skeleton } from "antd";
import { useEffect, useState } from "react";

import { getPreferences, updatePreferences } from "../api/preferences";
import type { TransactionsPeriodBasis } from "../api/contracts";
import { UnavailableState } from "../components/AsyncState";
import { Section } from "../components/layout";
import { SettingsPageContainer as PageContainer } from "../components/SettingsPageContainer";
import { useFeedback } from "../components/shared/useFeedback";

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

export function GeneralSettingsPage() {
  const [basis, setBasis] = useState<TransactionsPeriodBasis | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  const feedback = useFeedback();

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setLoadError(null);
    getPreferences(controller.signal)
      .then((preferences) => setBasis(preferences.transactions_period_basis))
      .catch((error) => {
        if (error instanceof DOMException && error.name === "AbortError") return;
        // A fixed sentence: whatever the transport said is not for the user.
        setLoadError("Não foi possível carregar as preferências.");
      })
      .finally(() => setLoading(false));
    return () => controller.abort();
  }, [attempt]);

  const save = async () => {
    if (basis === null) return;
    setSaving(true);
    setSaveError(null);
    try {
      await updatePreferences({ transactions_period_basis: basis });
      feedback.success("Preferência salva");
    } catch (error) {
      const message = errorMessage(error, "Não foi possível salvar as preferências.");
      setSaveError(message);
      feedback.error(message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <PageContainer title="Geral" subTitle="Preferências gerais do Julius" compactMobileHeader>
      <Section title="Mês das transações de cartão de crédito">
        {loading ? (
          <Skeleton active paragraph={{ rows: 2 }} />
        ) : loadError ? (
          <UnavailableState onRetry={() => setAttempt((count) => count + 1)}>{loadError}</UnavailableState>
        ) : (
          <div className="settings-form">
            <p className="settings-radio-hint">
              Escolha se as compras no cartão contam no mês em que foram feitas ou no mês da fatura em que são
              pagas. Transações de débito e de outras contas não têm essa distinção.
            </p>
            {saveError && <Alert type="error" showIcon message={saveError} />}
            <Radio.Group
              aria-label="Mês das transações de cartão de crédito"
              value={basis}
              onChange={(event) => setBasis(event.target.value as TransactionsPeriodBasis)}
            >
              <div className="settings-radio">
                <Radio value="occurred_at">
                  Mês da compra
                  <span className="settings-radio-hint">Cada compra conta no mês em que foi realizada.</span>
                </Radio>
                <Radio value="paid_at">
                  Mês do pagamento
                  <span className="settings-radio-hint">
                    Cada compra conta no mês da fatura em que é efetivamente paga. Compras recentes, cuja
                    fatura ainda não fechou, usam uma estimativa que se ajusta sozinha assim que a fatura
                    fecha numa sincronização seguinte.
                  </span>
                </Radio>
              </div>
            </Radio.Group>
            <Button type="primary" loading={saving} onClick={save}>
              Salvar
            </Button>
          </div>
        )}
      </Section>
    </PageContainer>
  );
}
