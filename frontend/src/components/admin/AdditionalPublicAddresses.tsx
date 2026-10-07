import { ArrowUpRight, Copy, X } from "lucide-react";
import { Dialog } from "@radix-ui/themes";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import AppDialogContent from "@/components/AppDialogContent";
import { writeClipboardText } from "@/utils/clipboard";
import { additionalPublicAddresses, type PublicIPAddress } from "@/utils/publicAddresses";

type Props = {
  addresses?: PublicIPAddress[];
  primaryIPv4?: string;
  primaryIPv6?: string;
  nodeName?: string;
};

export default function AdditionalPublicAddresses({ addresses, primaryIPv4, primaryIPv6, nodeName }: Props) {
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

  // Build and render the complete inventory only while the dialog is open.
  const inventory: PublicIPAddress[] = expanded ? [
    ...(primaryIPv4 ? [{ address: primaryIPv4, family: "ipv4" as const, primary: true }] : []),
    ...(primaryIPv6 ? [{ address: primaryIPv6, family: "ipv6" as const, primary: true }] : []),
    ...additional,
  ] : [];

  return (
    <Dialog.Root open={expanded} onOpenChange={setExpanded}>
      <Dialog.Trigger>
        <button type="button" className="admin-network-trigger" aria-label={`View all network addresses${nodeName ? ` for ${nodeName}` : ""}; ${additional.length} additional`}>
          <span className="admin-network-count">+{additional.length}</span>
          <span>addresses</span>
          <ArrowUpRight size={12} aria-hidden="true" />
        </button>
      </Dialog.Trigger>
      {expanded && <AppDialogContent
        className="admin-network-dialog"
        maxWidth="600px"
        title="Network addresses"
        description={nodeName ? `${nodeName} · ${inventory.length} public addresses` : `${inventory.length} public addresses`}
      >
        <Dialog.Close>
          <button type="button" className="admin-network-close" aria-label="Close network addresses"><X size={18} /></button>
        </Dialog.Close>
        <div className="admin-network-inventory">
          {(["ipv4", "ipv6"] as const).map((family) => {
            const entries = inventory.filter((entry) => entry.family === family);
            if (!entries.length) return null;
            return <section className="admin-network-family" key={family} aria-label={family === "ipv4" ? "IPv4 addresses" : "IPv6 addresses"}>
              <h3 className="admin-network-family-title">{family === "ipv4" ? "IPv4" : "IPv6"}<span>{entries.length}</span></h3>
              {entries.map(({ address, primary, interface: nic }) => (
                <div key={`${family}:${address}`} className="admin-network-entry">
                  <div className="min-w-0">
                    <span className="admin-address-value">{address}</span>
                    <span className="admin-network-meta">
                      <span className={primary ? "admin-network-primary-badge" : ""}>{primary ? "Primary" : "Additional"}</span>
                      {nic && <span>{nic}</span>}
                    </span>
                  </div>
                  <button type="button" className="admin-network-copy" onClick={() => copy(address)} aria-label={`${t("copy", "Copy")} ${address}`} title={t("copy", "Copy")}><Copy size={15} /></button>
                </div>
              ))}
            </section>;
          })}
        </div>
        <p className="admin-network-footnote">Reported by the Agent. Address availability does not indicate route reachability.</p>
      </AppDialogContent>}
    </Dialog.Root>
  );
}
