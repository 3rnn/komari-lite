import { DropdownMenu, IconButton, Text } from "@radix-ui/themes";
import { useContext, type ReactNode } from "react";
import {
  allowedColors,
  ThemeContext,
  type Colors,
} from "../contexts/ThemeContext";
import { BlendingModeIcon } from "@radix-ui/react-icons";
import { useTranslation } from "react-i18next";
import { useOptionalAccount } from "@/contexts/AccountContext";

interface ColorSwitchProps {
  icon?: ReactNode;
}

const ColorSwitch = ({ 
  icon = (
    <IconButton variant="soft">
      <BlendingModeIcon />
    </IconButton>
  ),
}: ColorSwitchProps = {}) => {
  const { setColor } = useContext(ThemeContext);
  const { t } = useTranslation();
  const accountContext = useOptionalAccount();

  const selectColor = (color: Colors) => {
    setColor(color);
    if (accountContext?.account?.logged_in) {
      void accountContext.updatePreferences({ color }).catch((error) => {
        console.warn("Failed to save color preference:", error);
      });
    }
  };

  return (
    <DropdownMenu.Root>
      <DropdownMenu.Trigger>
          {icon}
      </DropdownMenu.Trigger>
      <DropdownMenu.Content>
        {allowedColors.map((color) => (
          <DropdownMenu.Item key={color} onSelect={() => selectColor(color)}>
            <Text color={color}>{t(`color.${color}`)}</Text>
          </DropdownMenu.Item>
        ))}
      </DropdownMenu.Content>
    </DropdownMenu.Root>
  );
};

export default ColorSwitch;
