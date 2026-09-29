import React from "react";
import { useRPC2Call } from "./RPC2Context";

export type NodeBasicInfo = {
  /** Unique node ID. */
  uuid: string;
  /** Node name. */
  name: string;
  /** CPU model. */
  cpu_name: string;
  /** Virtualization. */
  virtualization: string;
  /** System architecture. */
  arch: string;
  /** CPU core count. */
  cpu_cores: number;
  /** Operating system. */
  os: string;
  /** Kernel version. */
  kernel_version: string;
  /** GPU model. */
  gpu_name: string;
  /** Region identifier. */
  region: string;
  region_override: string;
  /** Total memory in bytes. */
  mem_total: number;
  /** Total swap in bytes. */
  swap_total: number;
  /** Total disk space in bytes. */
  disk_total: number;
  /** Version. */
  version: string;
  /** Weight. */
  weight: number;
  /** Price. */
  price: number;
  tags: string;
  /** Billing period in days. */
  billing_cycle: number;
  /** Currency. */
  currency: string;
  /** Group. */
  group: string;
  /** Traffic threshold. */
  traffic_limit: number;
  /** Traffic threshold type. */
  traffic_limit_type: undefined | "sum" | "max" | "min" | "up" | "down";
  /** Monthly traffic reset day. 0 disables reset; null follows Agent config. */
  traffic_reset_day?: number | null;
  traffic_reset_allowance: number;
  effective_traffic_limit: number;
  effective_traffic_type: "sum" | "max" | "min" | "up" | "down";
  /** Expiration time. */
  expired_at: string;
  /** Creation time. */
  created_at: string;
  /** Last update time. */
  updated_at: string;
  ipv4?: string; 
  ipv6?: string;
};

interface NodeListContextType {
  nodeList: NodeBasicInfo[] | null;
  isLoading: boolean;
  error: string | null;
  refresh: () => void;
}

const NODE_LIST_CONTEXT_KEY = "__komariNodeListContext" as const;

type NodeListContextGlobal = typeof globalThis & {
  [NODE_LIST_CONTEXT_KEY]?: React.Context<NodeListContextType | undefined>;
};

const globalNodeListContext = globalThis as NodeListContextGlobal;
const NodeListContext =
  globalNodeListContext[NODE_LIST_CONTEXT_KEY] ??
  (globalNodeListContext[NODE_LIST_CONTEXT_KEY] =
    React.createContext<NodeListContextType | undefined>(undefined));

const sameNodeBasicInfo = (left: NodeBasicInfo, right: NodeBasicInfo) =>
  (Object.keys(right) as Array<keyof NodeBasicInfo>).every(
    (key) => left[key] === right[key],
  );

export const NodeListProvider: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => {
  const [nodeList, setNodeList] = React.useState<NodeBasicInfo[] | null>(null);
  const [isLoading, setIsLoading] = React.useState<boolean>(true);
  const [error, setError] = React.useState<string | null>(null);
  const { call } = useRPC2Call();
  const refreshSeqRef = React.useRef(0);
  const mountedRef = React.useRef(true);

  React.useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const refresh = React.useCallback(() => {
    const refreshSeq = ++refreshSeqRef.current;
    // setIsLoading(true);
    setError(null);
    // Fetch basic node information through RPC2.
    call<{ uuid?: string }, Record<string, any>>("common:getNodes")
      .then((result) => {
        if (!mountedRef.current || refreshSeq !== refreshSeqRef.current) return;
        if (!result || typeof result !== "object") {
          setNodeList([]);
          return;
        }
        // Convert { [uuid]: Client } into NodeBasicInfo[].
        const list: NodeBasicInfo[] = Object.values(result).map((n: any) => ({
          uuid: n.uuid,
          name: n.name,
          cpu_name: n.cpu_name,
          virtualization: n.virtualization,
          arch: n.arch,
          cpu_cores: n.cpu_cores,
          os: n.os,
          kernel_version: n.kernel_version,
          gpu_name: n.gpu_name,
          region: n.region,
          region_override: n.region_override ?? "",
          mem_total: n.mem_total,
          swap_total: n.swap_total,
          disk_total: n.disk_total,
          // Support older records by using an empty string when the version is absent.
          version: n.version ?? "",
          weight: n.weight ?? 0,
          price: n.price ?? 0,
          tags: n.tags ?? "",
          billing_cycle: n.billing_cycle ?? 0,
          currency: n.currency ?? "",
          group: n.group ?? "",
          traffic_limit: n.traffic_limit ?? 0,
          traffic_limit_type: n.traffic_limit_type,
          traffic_reset_day: n.traffic_reset_day ?? null,
          traffic_reset_allowance: n.traffic_reset_allowance ?? 0,
          effective_traffic_limit: n.effective_traffic_limit ?? n.traffic_limit ?? 0,
          effective_traffic_type: n.effective_traffic_type ?? n.traffic_limit_type ?? "sum",
          expired_at: n.expired_at ?? "",
          created_at: n.created_at ?? "",
          updated_at: n.updated_at ?? "",
          ipv4: n.ipv4,
          ipv6: n.ipv6,
        }));
        setNodeList((previous) => {
          if (!previous) return list;
          const previousByUuid = new Map(
            previous.map((node) => [node.uuid, node]),
          );
          let changed = previous.length !== list.length;
          const shared = list.map((node, index) => {
            const previousNode = previousByUuid.get(node.uuid);
            if (previousNode && sameNodeBasicInfo(previousNode, node)) {
              if (previous[index] !== previousNode) changed = true;
              return previousNode;
            }
            changed = true;
            return node;
          });
          return changed ? shared : previous;
        });
      })
      .catch((err: any) => {
        if (!mountedRef.current || refreshSeq !== refreshSeqRef.current) return;
        setError(err?.message || "An error occurred while fetching data");
        setNodeList([]);
      })
      .finally(() => {
        if (!mountedRef.current || refreshSeq !== refreshSeqRef.current) return;
        setIsLoading(false);
      });
  }, [call]);

  React.useEffect(() => {
    refresh();
  }, [refresh]);

  const contextValue = React.useMemo(
    () => ({ nodeList, isLoading, error, refresh }),
    [nodeList, isLoading, error, refresh],
  );

  return (
    <NodeListContext.Provider value={contextValue}>
      {children}
    </NodeListContext.Provider>
  );
};

export function useNodeList(): NodeListContextType;
export function useNodeList(required: false): NodeListContextType | undefined;
export function useNodeList(required = true) {
  const context = React.useContext(NodeListContext);
  if (!context && required) {
    throw new Error("useNodeList must be used within a NodeListProvider");
  }
  return context;
}
