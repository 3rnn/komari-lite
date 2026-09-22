import { Dialog, Text } from "@radix-ui/themes";
import { useTranslation } from "react-i18next";

import { usePublicInfo } from "@/contexts/PublicInfoContext";

type LoginIdentityHeaderProps = {
  dialog?: boolean;
};

export default function LoginIdentityHeader({
  dialog = false,
}: LoginIdentityHeaderProps) {
  const { t } = useTranslation();
  const { publicInfo } = usePublicInfo();
  const title = publicInfo?.sitename || "Komari Lite";

  return (
    <header className="mb-6 flex items-center gap-3">
      <img src="/favicon.ico" alt="" className="size-12 shrink-0 object-contain" />
      <div className="min-w-0">
        {dialog ? (
          <Dialog.Title size="5" mb="0" className="truncate">
            {title}
          </Dialog.Title>
        ) : (
          <Text as="div" size="5" weight="bold" className="truncate">
            {title}
          </Text>
        )}
        {dialog ? (
          <Dialog.Description size="2" color="gray">
            {t("login.desc")}
          </Dialog.Description>
        ) : (
          <Text as="div" size="2" color="gray">
            {t("login.desc")}
          </Text>
        )}
      </div>
    </header>
  );
}
