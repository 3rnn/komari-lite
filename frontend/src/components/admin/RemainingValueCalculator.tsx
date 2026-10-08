import React from "react";
import { Button, Callout, Flex, Text, TextField } from "@radix-ui/themes";
import { Copy, Minus, Plus } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { writeClipboardText } from "@/utils/clipboard";
import {
  buildResaleListing,
  calculateRemainingValue,
  formatCny,
  type ResaleAdjustment,
} from "@/lib/remainingValue";

type RemainingValueCalculatorProps = {
  open: boolean;
  nodeName: string;
  price: string;
  currency: string;
  billingCycle: string;
  expiredAt: string;
};

const emptyAdjustment = (): ResaleAdjustment => ({ kind: "fixed-cny", value: "0" });

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
  const [cycleStartedAt, setCycleStartedAt] = React.useState("");
  const [cnyPerUnit, setCnyPerUnit] = React.useState(currency === "¥" ? "1" : "");
  const [adjustments, setAdjustments] = React.useState<ResaleAdjustment[]>([]);

  React.useEffect(() => {
    if (open) return;
    setExpanded(false);
    setCycleStartedAt("");
    setCnyPerUnit(currency === "¥" ? "1" : "");
    setAdjustments([]);
  }, [currency, open]);

  React.useEffect(() => {
    if (currency === "¥" && !cnyPerUnit) setCnyPerUnit("1");
  }, [cnyPerUnit, currency]);

  const result = React.useMemo(() => calculateRemainingValue({
    price,
    billingCycle,
    expiredAt,
    cycleStartedAt,
    cnyPerUnit,
    adjustments,
  }), [adjustments, billingCycle, cnyPerUnit, cycleStartedAt, expiredAt, price]);

  const updateAdjustment = (index: number, update: Partial<ResaleAdjustment>) => {
    setAdjustments((current) => current.map((adjustment, adjustmentIndex) => (
      adjustmentIndex === index ? { ...adjustment, ...update } : adjustment
    )));
  };

  const copyListing = async () => {
    if (result.status !== "ready") return;
    try {
      const copied = await writeClipboardText(buildResaleListing({
        name: nodeName,
        sourceCurrency: currency,
        billingCycle: Number(billingCycle),
        expiredAt,
        cnyPerUnit: Number(cnyPerUnit),
        result,
      }));
      if (copied.confirmed) {
        toast.success(t("admin.nodeTable.resaleListingCopied", "Resale listing copied"));
      } else {
        toast.info(t("copy_unconfirmed", "Copy attempted; please verify your clipboard"));
      }
    } catch (error) {
      toast.error(`${t("admin.nodeTable.copyResaleListing", "Copy resale listing")}: ${error}`);
    }
  };

  const statusMessage: Record<typeof result.status, string> = {
    ready: "",
    "missing-input": t("admin.nodeTable.calculatorMissingInput", "Enter a billing price, cycle, expiry date, current cycle start date, and CNY rate."),
    "invalid-input": t("admin.nodeTable.calculatorInvalidInput", "Use a non-negative price, positive CNY rate, and valid dates."),
    "unsupported-cycle": t("admin.nodeTable.calculatorUnsupportedCycle", "One-time, zero, and negative billing cycles cannot be prorated."),
    "cycle-mismatch": t("admin.nodeTable.calculatorCycleMismatch", "The current cycle start and expiry do not match this billing cycle's renewal rules."),
    expired: t("admin.nodeTable.calculatorExpired", "This billing period has no remaining days."),
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
              {t("admin.nodeTable.calculatorHelp", "Calculator-only values are not saved with Billing. The cycle start is required so calendar billing periods are prorated exactly.")}
            </Text>
          </div>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <label className="flex min-w-0 flex-col gap-1 text-sm font-medium">
              {t("admin.nodeTable.currentCycleStart", "Current cycle start")}
              <TextField.Root type="date" value={cycleStartedAt} onChange={(event) => setCycleStartedAt(event.target.value)} />
            </label>
            <label className="flex min-w-0 flex-col gap-1 text-sm font-medium">
              {t("admin.nodeTable.cnyPerUnit", "CNY per currency unit")}
              <TextField.Root type="number" inputMode="decimal" min="0" step="any" value={cnyPerUnit} onChange={(event) => setCnyPerUnit(event.target.value)} />
            </label>
          </div>

          <div className="flex items-center justify-between gap-2">
            <Text size="2" weight="bold">{t("admin.nodeTable.resaleAdjustments", "Premiums and discounts")}</Text>
            <Button type="button" size="1" variant="ghost" onClick={() => setAdjustments((current) => [...current, emptyAdjustment()])}>
              <Plus size={14} /> {t("admin.nodeTable.addAdjustment", "Add adjustment")}
            </Button>
          </div>
          {adjustments.map((adjustment, index) => (
            <div key={index} className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] items-end gap-2">
              <label className="flex min-w-0 flex-col gap-1 text-xs font-medium text-muted-foreground">
                {t("admin.nodeTable.adjustmentType", "Type")}
                <select
                  className="h-8 min-w-0 rounded-md border border-[var(--gray-a6)] bg-[var(--color-panel-solid)] px-2 text-sm text-[var(--gray-12)]"
                  value={adjustment.kind}
                  onChange={(event) => updateAdjustment(index, { kind: event.target.value as ResaleAdjustment["kind"] })}
                >
                  <option value="fixed-cny">{t("admin.nodeTable.fixedCny", "Fixed CNY")}</option>
                  <option value="percent">{t("admin.nodeTable.percent", "Percentage")}</option>
                </select>
              </label>
              <label className="flex min-w-0 flex-col gap-1 text-xs font-medium text-muted-foreground">
                {adjustment.kind === "percent" ? t("admin.nodeTable.percent", "Percentage") : t("admin.nodeTable.amount", "Amount")}
                <TextField.Root type="number" inputMode="decimal" step="any" value={adjustment.value} onChange={(event) => updateAdjustment(index, { value: event.target.value })} />
              </label>
              <Button type="button" size="1" color="red" variant="ghost" aria-label={t("admin.nodeTable.removeAdjustment", "Remove adjustment")} onClick={() => setAdjustments((current) => current.filter((_, adjustmentIndex) => adjustmentIndex !== index))}>
                <Minus size={15} />
              </Button>
            </div>
          ))}

          {result.status === "ready" ? (
            <div className="grid grid-cols-1 gap-2 border-t border-[var(--gray-a5)] pt-3 sm:grid-cols-2">
              <CalculatorMetric label={t("admin.nodeTable.remainingDays", "Remaining days")} value={`${result.remainingDays} / ${result.cycleDays}`} />
              <CalculatorMetric label={t("admin.nodeTable.remainingValue", "Remaining value")} value={formatCny(result.remainingValueCny!)} />
              <CalculatorMetric label={t("admin.nodeTable.finalResalePrice", "Final resale price")} value={formatCny(result.resalePriceCny!)} emphasized />
              <CalculatorMetric label={t("admin.nodeTable.resalePriceSource", "Resale price in source currency")} value={`${currency}${result.resalePriceSource!.toFixed(2)}`} />
              <Button type="button" variant="outline" className="sm:col-span-2" onClick={copyListing}>
                <Copy size={15} /> {t("admin.nodeTable.copyResaleListing", "Copy resale listing")}
              </Button>
            </div>
          ) : (
            <Callout.Root color={result.status === "expired" ? "orange" : "gray"} size="1">
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
