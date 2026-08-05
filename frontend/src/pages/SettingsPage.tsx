import { PageContainer } from "@ant-design/pro-layout";
import { Alert, Button, Card, Flex, Radio, Skeleton, Typography } from "antd";
import { useEffect, useState } from "react";

import { getPreferences, updatePreferences } from "../api/preferences";
import type { TransactionsPeriodBasis } from "../api/contracts";

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Não foi possível salvar as preferências.";
}

export function SettingsPage() {
  const [basis, setBasis] = useState<TransactionsPeriodBasis | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setLoadError(null);
    getPreferences(controller.signal)
      .then((preferences) => setBasis(preferences.transactions_period_basis))
      .catch((error) => {
        if (error instanceof DOMException && error.name === "AbortError") return;
        setLoadError(errorMessage(error));
      })
      .finally(() => setLoading(false));
    return () => controller.abort();
  }, []);

  const save = async () => {
    if (basis === null) return;
    setSaving(true);
    setSaveError(null);
    setSaved(false);
    try {
      await updatePreferences({ transactions_period_basis: basis });
      setSaved(true);
    } catch (error) {
      setSaveError(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  return (
    <PageContainer
      title="Configurações"
      subTitle="Preferências gerais do Contadinho"
    >
      <Card title="Mês das transações de cartão de crédito">
        {loading ? (
          <Skeleton active paragraph={{ rows: 2 }} />
        ) : loadError ? (
          <Alert type="error" showIcon message={loadError} />
        ) : (
          <Flex vertical gap="middle">
            <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
              Escolha se as compras no cartão de crédito devem contar no mês em que foram realizadas
              ou no mês em que efetivamente entram na fatura paga. Essa escolha não afeta transações de
              débito ou outras contas, que não têm essa distinção.
            </Typography.Paragraph>
            {saveError && <Alert type="error" showIcon message={saveError} />}
            {saved && <Alert type="success" showIcon message="Preferência salva." closable onClose={() => setSaved(false)} />}
            <Radio.Group
              value={basis}
              onChange={(event) => {
                setBasis(event.target.value as TransactionsPeriodBasis);
                setSaved(false);
              }}
            >
              <Flex vertical gap="small">
                <Radio value="occurred_at">
                  Mês da compra
                  <div>
                    <Typography.Text type="secondary">
                      Cada compra conta no mês em que foi realizada (comportamento atual).
                    </Typography.Text>
                  </div>
                </Radio>
                <Radio value="paid_at">
                  Mês do pagamento
                  <div>
                    <Typography.Text type="secondary">
                      Cada compra conta no mês da fatura em que é efetivamente paga. Compras recentes,
                      cuja fatura ainda não fechou, usam uma estimativa que se ajusta sozinha assim que a
                      fatura fecha numa sincronização seguinte.
                    </Typography.Text>
                  </div>
                </Radio>
              </Flex>
            </Radio.Group>
            <Button type="primary" loading={saving} onClick={save} style={{ alignSelf: "flex-start" }}>
              Salvar
            </Button>
          </Flex>
        )}
      </Card>
    </PageContainer>
  );
}
