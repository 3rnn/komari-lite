import React from "react";
import {
  Button,
  Callout,
  Flex,
  IconButton,
  Select,
  Text,
  TextField,
} from "@radix-ui/themes";
import { Copy, RefreshCw } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import {
  fetchCnyExchangeRate,
  resolveBillingCurrencyCode,
  type ExchangeRateResult,
} from "@/lib/exchangeRates";
import {
  buildVpsJsqListing,
  calculateVpsJsqRemainingValue,
  formatCny,
  getVpsJsqDefaultCnyRate,
  todayVpsJsqDateInput,
  type VpsJsqAdjustmentMode,
} from "@/lib/remainingValue";
import { writeClipboardText } from "@/utils/clipboard";

type RemainingValueCalculatorProps = {
  open: boolean;
  nodeName: string;
  price: string;
  currency: string;
  billingCycle: string;
  expiredAt: string;
};

type RateStatus = ExchangeRateResult | { kind: "loading" } | undefined;

function formatUpdateTime(timestamp: number): string {
  return new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(new Date(timestamp));
}

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
  const [rateStatus, setRateStatus] = React.useState<RateStatus>();
  const [manualRateOverride, setManualRateOverride] = React.useState(false);
  const manualRateOverrideRef = React.useRef(false);
  const requestId = React.useRef(0);
  const wasOpen = React.useRef(false);
  const currencyCode = resolveBillingCurrencyCode(currency);
  const isCny = currencyCode === "CNY";

  const loadRate = React.useCallback(async (force = false) => {
    const id = ++requestId.current;
    setRateStatus({ kind: "loading" });
    const loaded = await fetchCnyExchangeRate(currency, { force });
    if (id !== requestId.current) return;

    setRateStatus(loaded);
    if (!manualRateOverrideRef.current || loaded.kind === "fixed") {
      if (loaded.kind === "fixed" || loaded.kind === "live" || loaded.kind === "cached" || loaded.kind === "stale-cache") {
        setCnyPerUnit(loaded.cnyPerUnit);
      }
    }
  }, [currency]);

  React.useEffect(() => {
    if (!open) {
      requestId.current += 1;
      wasOpen.current = false;
      setExpanded(false);
      return;
    }

    const opening = !wasOpen.current;
    wasOpen.current = true;
    if (opening) {
      setTransactionDate(todayVpsJsqDateInput());
      setAdjustmentMode("add");
      setAdjustmentValue("0");
    }
    manualRateOverrideRef.current = false;
    setManualRateOverride(false);
    setCnyPerUnit(getVpsJsqDefaultCnyRate(currency));
    void loadRate();
  }, [currency, loadRate, open]);

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
    "invalid-input": t("admin.nodeTable.calculatorInvalidInput", "Use numeric values and a positive CNY rate."),
    "unsupported-cycle": t("admin.nodeTable.calculatorUnsupportedCycle", "A positive billing-cycle day count is required."),
  };

  const rateMessage = (() => {
    if (!rateStatus) return "";
    if (rateStatus.kind === "loading") return t("admin.nodeTable.exchangeRateLoading", "Loading ExchangeRate-API rate…");
    if (rateStatus.kind === "fixed") return t("admin.nodeTable.exchangeRateCny", "CNY conversion is fixed at 1.");
    if (rateStatus.kind === "live") return t("admin.nodeTable.exchangeRateLive", "ExchangeRate-API · Updated {{time}}", { time: formatUpdateTime(rateStatus.updatedAt) });
    if (rateStatus.kind === "cached") return t("admin.nodeTable.exchangeRateCached", "ExchangeRate-API · Cached, updated {{time}}", { time: formatUpdateTime(rateStatus.updatedAt) });
    if (rateStatus.kind === "stale-cache") return t("admin.nodeTable.exchangeRateStale", "ExchangeRate-API unavailable · Using cached rate from {{time}}", { time: formatUpdateTime(rateStatus.updatedAt) });
    if (rateStatus.kind !== "error") return "";
    if (rateStatus.reason === "rate-limited") return t("admin.nodeTable.exchangeRateRateLimited", "ExchangeRate-API rate limit reached. Enter a rate manually or try again later.");
    if (rateStatus.reason === "timeout") return t("admin.nodeTable.exchangeRateTimeout", "ExchangeRate-API timed out. Enter a rate manually or try again.");
    if (rateStatus.reason === "unsupported-currency") return t("admin.nodeTable.exchangeRateUnsupported", "No ExchangeRate-API currency code is available. Enter a rate manually.");
    return t("admin.nodeTable.exchangeRateUnavailable", "ExchangeRate-API is unavailable. Enter a rate manually or refresh.");
  })();

  return (
    <div className="border-t border-[var(--gray-a5)] pt-3">
      <Button type="button" variant="soft" onClick={() => setExpanded((current) => !current)}>
        {t("admin.nodeTable.remainingValueCalculator", "Remaining Value Calculator")}
      </Button>
      {expanded ? (
        <Flex direction="column" gap="3" className="mt-3 rounded-md border border-[var(--gray-a5)] bg-[var(--gray-a2)] p-3">
          <Text as="div" size="1" weight="bold" className="uppercase tracking-wide text-muted-foreground">
            {t("admin.nodeTable.remainingValueCalculator", "Remaining Value Calculator")}
          </Text>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <label className="flex min-w-0 flex-col gap-1 text-sm font-medium">
              {t("admin.nodeTable.transactionDate", "Transaction date")}
              <TextField.Root type="date" value={transactionDate} onChange={(event) => setTransactionDate(event.target.value)} />
            </label>
            <div className="min-w-0">
              <label className="flex min-w-0 flex-col gap-1 text-sm font-medium">
                {t("admin.nodeTable.cnyPerUnit", "CNY per currency unit")}
                <TextField.Root
                  type="number"
                  inputMode="decimal"
                  min="0"
                  step="any"
                  value={cnyPerUnit}
                  disabled={isCny}
                  onChange={(event) => {
                    manualRateOverrideRef.current = true;
                    setManualRateOverride(true);
                    setCnyPerUnit(event.target.value);
                  }}
                >
                  <TextField.Slot side="right">
                    <IconButton
                      type="button"
                      size="1"
                      variant="ghost"
                      aria-label={t("admin.nodeTable.refreshExchangeRate", "Refresh exchange rate")}
                      title={t("admin.nodeTable.refreshExchangeRate", "Refresh exchange rate")}
                      disabled={isCny || rateStatus?.kind === "loading"}
                      onClick={() => {
                        manualRateOverrideRef.current = false;
                        setManualRateOverride(false);
                        void loadRate(true);
                      }}
                    >
                      <RefreshCw size={14} className={rateStatus?.kind === "loading" ? "animate-spin" : undefined} />
                    </IconButton>
                  </TextField.Slot>
                </TextField.Root>
              </label>
              <Text as="div" size="1" color="gray" className="mt-1 min-h-4">
                {manualRateOverride ? `${t("admin.nodeTable.exchangeRateManual", "Manual rate override")} · ${rateMessage}` : rateMessage}
              </Text>
              {!isCny ? (
                <Text as="div" size="1" color="gray" className="mt-1">
                  <a href="https://www.exchangerate-api.com" target="_blank" rel="noreferrer">Exchange rates by ExchangeRate-API</a>
                </Text>
              ) : null}
            </div>
          </div>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <label className="flex min-w-0 flex-col gap-1 text-sm font-medium">
              {t("admin.nodeTable.priceAdjustment", "Price adjustment (CNY)")}
              <Select.Root value={adjustmentMode} onValueChange={(value) => setAdjustmentMode(value as VpsJsqAdjustmentMode)}>
                <Select.Trigger className="w-full" />
                <Select.Content>
                  <Select.Item value="add">{t("admin.nodeTable.fixedPremium", "Fixed premium")}</Select.Item>
                  <Select.Item value="sub">{t("admin.nodeTable.fixedDiscount", "Fixed discount")}</Select.Item>
                  <Select.Item value="target">{t("admin.nodeTable.targetPrice", "Target price")}</Select.Item>
                </Select.Content>
              </Select.Root>
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
