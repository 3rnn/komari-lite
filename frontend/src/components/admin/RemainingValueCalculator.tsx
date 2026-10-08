import React from "react";
import { Button, Callout, Flex, Text, TextField } from "@radix-ui/themes";
import { Copy } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { writeClipboardText } from "@/utils/clipboard";
import {
  buildVpsJsqListing,
  calculateVpsJsqRemainingValue,
  formatCny,
  getVpsJsqDefaultCnyRate,
  todayVpsJsqDateInput,
  type VpsJsqAdjustmentMode,
} from "@/lib/remainingValue";

type RemainingValueCalculatorProps = {
  open: boolean;
  nodeName: string;
  price: string;
  currency: string;
  billingCycle: string;
  expiredAt: string;
};

export default function RemainingValueCalculator({
  open,
  nodeName,
  price,
  currency,
  billingCycle,
  expiredAt,
}: RemainingValueCalculatorProps) {
  const { t } = useTranslation();
  const [expanded, setExpanded] = React.useState(false);
  const [transactionDate, setTransactionDate] = React.useState(() => todayVpsJsqDateInput());
  const [cnyPerUnit, setCnyPerUnit] = React.useState(() => getVpsJsqDefaultCnyRate(currency));
  const [adjustmentMode, setAdjustmentMode] = React.useState<VpsJsqAdjustmentMode>("add");
  const [adjustmentValue, setAdjustmentValue] = React.useState("0");

  React.useEffect(() => {
    if (!open) {
      setExpanded(false);
      return;
    }
    setTransactionDate(todayVpsJsqDateInput());
    setCnyPerUnit(getVpsJsqDefaultCnyRate(currency));
    setAdjustmentMode("add");
    setAdjustmentValue("0");
  }, [currency, open]);

  const result = React.useMemo(() => calculateVpsJsqRemainingValue({
    price,
    billingCycle,
    expiredAt,
    transactionDate,
    cnyPerUnit,
    adjustmentMode,
    adjustmentValue,
  }), [adjustmentMode, adjustmentValue, billingCycle, cnyPerUnit, expiredAt, price, transactionDate]);

  const copyListing = async () => {
    if (result.status !== "ready") return;
    try {
      const copied = await writeClipboardText(buildVpsJsqListing({
        sourceCurrency: currency,
        price,
        billingCycle,
        expiredAt,
        transactionDate,
        cnyPerUnit,
        result,
      }));
      if (copied.confirmed) {
        toast.success(t("admin.nodeTable.resaleListingCopied", "Resale listing copied"));
      } else {
        toast.info(t("copy_unconfirmed", "Copy attempted; please verify your clipboard"));
      }
    } catch (error) {
      toast.error(`${nodeName}: ${t("admin.nodeTable.copyResaleListing", "Copy resale listing")}: ${error}`);
    }
  };

  const statusMessage: Record<typeof result.status, string> = {
    ready: "",
    "missing-input": t("admin.nodeTable.calculatorMissingInput", "Enter a billing cycle, expiration date, transaction date, and CNY rate."),
    "invalid-input": t("admin.nodeTable.calculatorInvalidInput", "Use numeric values and valid dates."),
    "unsupported-cycle": t("admin.nodeTable.calculatorUnsupportedCycle", "A positive billing-cycle day count is required."),
  };

  return (
    <div className="border-t border-[var(--gray-a5)] pt-3">
      <Button type="button" variant="soft" onClick={() => setExpanded((current) => !current)}>
        {t("admin.nodeTable.remainingValueCalculator", "Remaining Value Calculator")}
      </Button>
      {expanded ? (
        <Flex direction="column" gap="3" className="mt-3 rounded-md border border-[var(--gray-a5)] bg-[var(--gray-a2)] p-3">
          <div>
            <Text as="div" size="1" weight="bold" className="uppercase tracking-wide text-muted-foreground">
              {t("admin.nodeTable.remainingValueCalculator", "Remaining Value Calculator")}
            </Text>
            <Text as="div" size="1" color="gray" className="mt-1">
              {t("admin.nodeTable.calculatorHelp", "Uses the VPS-JSQ day-count formula. Calculator-only values are not saved with Billing.")}
            </Text>
          </div>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <label className="flex min-w-0 flex-col gap-1 text-sm font-medium">
              {t("admin.nodeTable.transactionDate", "Transaction date")}
              <TextField.Root type="date" value={transactionDate} onChange={(event) => setTransactionDate(event.target.value)} />
            </label>
            <label className="flex min-w-0 flex-col gap-1 text-sm font-medium">
              {t("admin.nodeTable.cnyPerUnit", "CNY per currency unit")}
              <TextField.Root type="number" inputMode="decimal" step="0.0001" value={cnyPerUnit} onChange={(event) => setCnyPerUnit(event.target.value)} />
            </label>
          </div>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
            <label className="flex min-w-0 flex-col gap-1 text-sm font-medium">
              {t("admin.nodeTable.priceAdjustment", "Price adjustment (CNY)")}
              <select
                className="h-9 min-w-0 rounded-md border border-[var(--gray-a6)] bg-[var(--color-panel-solid)] px-2 text-sm text-[var(--gray-12)]"
                value={adjustmentMode}
                onChange={(event) => setAdjustmentMode(event.target.value as VpsJsqAdjustmentMode)}
              >
                <option value="add">{t("admin.nodeTable.fixedPremium", "Fixed premium")}</option>
                <option value="sub">{t("admin.nodeTable.fixedDiscount", "Fixed discount")}</option>
                <option value="target">{t("admin.nodeTable.targetPrice", "Target price")}</option>
              </select>
            </label>
            <label className="flex min-w-0 flex-col gap-1 text-sm font-medium">
              {adjustmentMode === "target"
                ? t("admin.nodeTable.targetPrice", "Target price")
                : t("admin.nodeTable.amount", "Amount")}
              <TextField.Root type="number" inputMode="decimal" step="any" value={adjustmentValue} onChange={(event) => setAdjustmentValue(event.target.value)} />
            </label>
          </div>

          {result.status === "ready" ? (
            <div className="grid grid-cols-1 gap-2 border-t border-[var(--gray-a5)] pt-3 sm:grid-cols-2">
              <CalculatorMetric label={t("admin.nodeTable.remainingDays", "Remaining days")} value={`${result.remainingDays} days`} />
              <CalculatorMetric label={t("admin.nodeTable.remainingPercentage", "Remaining percentage")} value={`${result.remainingPercentage!.toFixed(1)}% / ${billingCycle} days`} />
              <CalculatorMetric label={t("admin.nodeTable.remainingValue", "Remaining value")} value={formatCny(result.baseResidualValueCny!)} />
              <CalculatorMetric label={t("admin.nodeTable.premiumDiscount", "Premium / discount")} value={`${result.premiumOrDiscountCny! > 0 ? "+" : ""}${result.premiumOrDiscountCny!.toFixed(2)}`} />
              <CalculatorMetric label={t("admin.nodeTable.finalResalePrice", "Final resale price")} value={formatCny(result.finalResalePriceCny!)} emphasized />
              <Button type="button" variant="outline" className="sm:col-span-2" onClick={copyListing}>
                <Copy size={15} /> {t("admin.nodeTable.copyResaleListing", "Copy resale listing")}
              </Button>
            </div>
          ) : (
            <Callout.Root color="gray" size="1">
              <Callout.Text>{statusMessage[result.status]}</Callout.Text>
            </Callout.Root>
          )}
        </Flex>
      ) : null}
    </div>
  );
}

function CalculatorMetric({ label, value, emphasized = false }: { label: string; value: string; emphasized?: boolean }) {
  return (
    <div className="min-w-0 border-l-2 border-[var(--accent-a7)] pl-2">
      <Text as="div" size="1" color="gray">{label}</Text>
      <Text as="div" size="3" weight={emphasized ? "bold" : "medium"} className="truncate tabular-nums">{value}</Text>
    </div>
  );
}
