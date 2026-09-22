import { Flex, Spinner, Text } from "@radix-ui/themes";
import { useTranslation } from "react-i18next";

const FullPageLoading = () => {
  const { t } = useTranslation();

  return (
    <Flex
      direction="column"
      align="center"
      justify="center"
      gap="3"
      style={{ minHeight: "100dvh", backgroundColor: "var(--accent-1)" }}
    >
      <Spinner size="3" />
      <Text size="2" color="gray">
        {t("loading")}
      </Text>
    </Flex>
  );
};

export default FullPageLoading;
