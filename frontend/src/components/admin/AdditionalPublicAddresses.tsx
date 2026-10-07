import { Copy } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { writeClipboardText } from "@/utils/clipboard";
import { additionalPublicAddresses, type PublicIPAddress } from "@/utils/publicAddresses";

type Props = {
  addresses?: PublicIPAddress[];
  primaryIPv4?: string;
  primaryIPv6?: string;
};

export default function AdditionalPublicAddresses({ addresses, primaryIPv4, primaryIPv6 }: Props) {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState(false);
  const additional = additionalPublicAddresses(addresses, primaryIPv4, primaryIPv6);
  if (additional.length === 0) return null;

  async function copy(address: string) {
    try {
      const result = await writeClipboardText(address);
      if (result.confirmed) {
        toast.success(t("copy_success"));
      } else {
        toast.info(t("copy_unconfirmed", "Copy attempted; please verify your clipboard"));
      }
    } catch {
      toast.error(t("copy_failed", "Could not copy address"));
    }
  }

  return (
    <details
      className="admin-additional-addresses min-w-0 max-w-full text-xs text-muted-foreground"
      onToggle={(event) => setExpanded(event.currentTarget.open)}
    >
      <summary className="cursor-pointer select-none py-0.5 text-[var(--accent-11)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--accent-9)]">
        {t("admin.nodeDetail.additionalAddresses", "Additional addresses")} ({additional.length})
      </summary>
      {expanded && <div className="admin-additional-address-list mt-1 rounded border border-[var(--gray-a5)] p-1.5">
        {additional.map(({ address, family, interface: nic }) => (
          <div key={`${family}:${address}`} className="admin-address-row py-0.5">
            <div className="min-w-0">
              <span className="admin-address-value" title={address}>{address}</span>
              <span className="admin-address-label">
                {family === "ipv4" ? "IPv4" : "IPv6"}{nic ? ` · ${nic}` : ""}
              </span>
            </div>
            <button
              type="button"
              className="inline-flex size-5 shrink-0 items-center justify-center rounded hover:bg-[var(--accent-a3)] hover:text-[var(--accent-11)]"
              onClick={() => copy(address)}
              aria-label={`${t("copy", "Copy")} ${address}`}
              title={t("copy", "Copy")}
            >
              <Copy size={13} />
            </button>
          </div>
        ))}
      </div>}
    </details>
  );
}
