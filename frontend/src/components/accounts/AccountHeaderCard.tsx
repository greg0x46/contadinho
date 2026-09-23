import { Card, Tag, Tooltip, Typography } from "antd";

import type { Account } from "../../api/contracts";
import { formatOptionalDate, formatOptionalDay } from "../../presentation/dates";
import {
  accountDisplayName,
  accountSubtypeLabel,
  accountTypeLabel,
  formatAccountMoney,
} from "../../presentation/accountLabels";
import { CreditUsageMeter } from "./CreditUsageMeter";
import { ClosingDayField } from "./ClosingDayField";

export function AccountHeaderCard({
  account,
  onSaveClosingDay,
  savingClosingDay,
}: {
  account: Account;
  onSaveClosingDay: (closingDay: number | null) => Promise<unknown>;
  savingClosingDay: boolean;
}) {
  const isCredit = account.account_type === "CREDIT";
  const subtype = accountSubtypeLabel(account.account_subtype);

  return (
    <Card className="debt-header-card dashboard-widget">
      <div className="debt-header-identity">
        <Typography.Title level={3} className="debt-header-name">
          {accountDisplayName(account)}
        </Typography.Title>
        <Tag>{accountTypeLabel(account.account_type)}</Tag>
        {account.institution !== null && (
          <Typography.Text type="secondary">{account.institution}</Typography.Text>
        )}
        {subtype !== null && <Typography.Text type="secondary">{subtype}</Typography.Text>}
      </div>

      <div className="debt-header-figure">
        <Tooltip
          title={
            isCredit
              ? "Saldo devedor informado pela instituição, incluindo a fatura ainda em aberto"
              : undefined
          }
        >
          <p className="dashboard-hero-figure">
            {formatAccountMoney(account.balance, account.currency_code)}
            {isCredit ? " em aberto" : " de saldo"}
          </p>
        </Tooltip>

        {isCredit && <CreditUsageMeter ratio={account.credit_usage_ratio} />}

        <ul className="debt-header-legend-inline">
          {isCredit && (
            <>
              <li>
                <span>Limite total</span>
                <strong>{formatAccountMoney(account.credit_limit, account.currency_code)}</strong>
              </li>
              <li>
                <span>Limite disponível</span>
                <strong>
                  {formatAccountMoney(account.available_credit_limit, account.currency_code)}
                </strong>
              </li>
              <ClosingDayField
                account={account}
                onSave={onSaveClosingDay}
                saving={savingClosingDay}
              />
              <li>
                <span>Vencimento</span>
                <strong>{formatOptionalDay(account.balance_due_date)}</strong>
              </li>
            </>
          )}
          {account.number !== null && (
            <li>
              <span>Número</span>
              <strong>{account.number}</strong>
            </li>
          )}
          {account.currency_code !== null && (
            <li>
              <span>Moeda</span>
              <strong>{account.currency_code}</strong>
            </li>
          )}
          <li>
            <span>Atualizado em</span>
            <strong>{formatOptionalDate(account.provider_updated_at ?? account.updated_at)}</strong>
          </li>
        </ul>
      </div>
    </Card>
  );
}
