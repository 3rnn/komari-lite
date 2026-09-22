import { Box, Flex, Skeleton } from "@radix-ui/themes";

const SettingsPageSkeleton = () => (
  <Flex
    aria-label="设置加载中"
    data-admin-route-pending="true"
    direction="column"
    gap="3"
    role="status"
    className="w-full"
  >
    <Skeleton width="9rem" height="1.75rem" />
    {[0, 1, 2].map((item) => (
      <Box
        key={item}
        className="min-h-20 rounded-md border bg-[var(--color-panel-solid)] p-4"
        style={{ borderColor: "var(--gray-a5)" }}
      >
        <Flex direction="column" gap="2">
          <Skeleton width={item === 1 ? "11rem" : "8rem"} height="1.1rem" />
          <Skeleton width="min(24rem, 75%)" height="0.85rem" />
        </Flex>
      </Box>
    ))}
  </Flex>
);

export default SettingsPageSkeleton;
