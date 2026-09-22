import { Flex } from "@radix-ui/themes";

import ColorSwitch from "./ColorSwitch";
import LanguageSwitch from "./Language";
import ThemeSwitch from "./ThemeSwitch";
import KomariLiteBrand from "./KomariLiteBrand";
import { getAppAssetUrl } from "@/utils/assetUrl";

export default function GuideHeader() {
  return (
    <Flex justify="between" align="center" gap="4" className="w-full">
      <Flex align="center" gap="2">
        <img
          src={getAppAssetUrl("assets/logo.png?v=869680cc")}
          alt="Komari Lite"
          className="size-9 object-contain"
        />
        <KomariLiteBrand size="sm" />
      </Flex>
      <Flex gap="2">
        <LanguageSwitch />
        <ThemeSwitch />
        <ColorSwitch />
      </Flex>
    </Flex>
  );
}
