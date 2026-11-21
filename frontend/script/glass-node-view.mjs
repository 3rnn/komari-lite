// Injected into the pinned Glass module with its React runtime and formatters.
function rW() {
  const { nodes, metricRetention } = eB();
  const [view, setView] = R.useState("list");
  const [search, setSearch] = R.useState("");
  R.useEffect(() => {
    try {
      const saved = localStorage.getItem("komari-glass-node-view");
      if (saved === "cards" || saved === "list") setView(saved);
    } catch { /* The default view is usable with storage disabled. */ }
  }, []);
  function selectView(next) {
    setView(next);
    try { localStorage.setItem("komari-glass-node-view", next); } catch { /* Optional preference. */ }
  }
  const query = search.trim().toLowerCase();
  const visibleNodes = query
    ? nodes.filter(node => node.name.toLowerCase().includes(query))
    : nodes;
  const pingEnabled = metricRetention.pingHours >= 1;

  return L.jsxs("div", {
    className: "glass-node-home",
    "data-node-view": view,
    children: [
      L.jsx(rH, {}),
      L.jsxs("section", { "aria-labelledby": "glass-nodes", children: [
        L.jsxs("div", {
          className: "node-view-toolbar",
          children: [
            L.jsx("h2", { id: "glass-nodes", className: "glass-section-title", children: "Nodes" }),
            L.jsxs("div", { className: "node-view-controls", children: [
              L.jsx("input", { className: "node-view-search", type: "search", "aria-label": "Find a node",
                placeholder: "Find a node", value: search, onChange: event => setSearch(event.target.value) }),
              L.jsx("div", { className: "node-view-switch", role: "group", "aria-label": "Node view", children:
                [["list", "List"], ["cards", "Cards"]].map(([value, label]) => L.jsx("button", {
                  type: "button", "aria-pressed": value === view, onClick: () => selectView(value), children: label,
                }, value)),
              }),
            ] }),
          ],
        }),
        L.jsx(rz, { visibleNodes, view, pingEnabled }),
      ] }),
    ],
  });
}

function glassNodeTable({ nodes, settings, pingEnabled, goInstance }) {
  const columns = [
    ["Node", ""], ["Uptime", "node-cell-secondary"], ["CPU", "node-cell-metric"],
    ["Memory", "node-cell-metric"], ["Disk", "node-cell-metric"],
    ["Traffic", "node-cell-traffic"],
    ...(pingEnabled ? [["Latency · 1h avg", "node-cell-latency"], ["Loss · 1h", ""]] : []),
  ];
  return L.jsx("div", { className: "node-table-panel", children: L.jsxs("table", {
    className: "node-table", "aria-labelledby": "glass-nodes", children: [
      L.jsx("thead", { children: L.jsx("tr", { children: columns.map(([label, className]) =>
        L.jsx("th", { scope: "col", className, children: label }, label)),
      }) }),
      L.jsx("tbody", { children: nodes.map(node =>
        L.jsx(GlassNodeRow, { node, settings, pingEnabled, goInstance }, node.uuid)),
      }),
    ],
  }) });
}

function GlassNodeRow({ node, settings, pingEnabled, goInstance }) {
  const selection = resolveNodePingSelection(settings.showCarrierPing, { preferredId: settings.preferredPingTaskId },
    node.display_ping_task_ids, node.display_ping_task_id);
  const ping = useGlassNodePing(node.uuid, pingEnabled, 1, selection);
  const tasks = ping.showAll ? ping.tasks : [{
    id: "selected", name: ping.selectedName, avgLatency: ping.avgLatency,
    latencyDisplay: ping.latencyDisplay, lossDisplay: ping.lossDisplay,
  }];
  const valueCell = (value, available = true) => {
    const percent = node.online && available && Number.isFinite(value) ? value : null;
    return L.jsxs("td", { className: "node-cell-metric", children: [
      L.jsx("span", { children: percent === null ? "—" : `${percent.toFixed(1)}%` }),
      percent === null ? null : L.jsx("span", { className: "node-meter", "aria-hidden": true, children:
        L.jsx("span", { style: { width: `${eA(percent)}%` } }),
      }),
    ] });
  };
  const uptime = node.online && Number.isFinite(node.uptime) && node.uptime >= 0
    ? `${Math.floor(node.uptime / 86400)}d ${Math.floor(node.uptime / 3600) % 24}h`
    : "—";
  const traffic = eE(node.net_total_up, node.net_total_down, node.traffic_limit_type);
  const hasTraffic = (node.online || node.updated_at_live) && Number.isFinite(traffic) && traffic >= 0;
  const hasQuota = node.traffic_limit > 0;
  const trafficPercent = hasQuota ? eA(eS(traffic, node.traffic_limit)) : null;
  const probeValues = (valueKey) => tasks.length ? tasks.map(task => L.jsxs("span", {
    className: "node-probe-value", children: [
      task.name && (valueKey === "latencyDisplay" || ping.showAll)
        ? L.jsx("span", { className: "node-probe-name", title: task.name, children: task.name }) : null,
      L.jsx("span", { className: valueKey === "latencyDisplay" && task.avgLatency > 200 ? "node-latency-high" : undefined,
        children: task[valueKey] === "-" ? "—" : task[valueKey] }),
    ],
  }, task.id)) : (ping.loading ? "Loading" : "—");
  return L.jsxs("tr", { children: [
    L.jsx("th", { scope: "row", children: L.jsxs("a", {
      className: "node-table-link", href: `/instance/${encodeURIComponent(node.uuid)}`,
      "aria-label": `View ${node.name} details`,
      onClick: event => {
        if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
        event.preventDefault();
        goInstance(node.uuid);
      },
      children: [
        L.jsx("span", { className: "node-status-dot", role: "img", "aria-label": node.online ? "Online" : "Offline",
          "data-online": node.online }),
        L.jsx("span", { className: "node-table-name", title: node.name, children: node.name }),
        L.jsx("span", { className: "glass-spearhead", "aria-hidden": true }),
      ],
    }) }),
    L.jsx("td", { className: "node-cell-secondary", children: uptime }),
    valueCell(node.cpu),
    valueCell(eS(node.ram, node.mem_total), node.mem_total > 0),
    valueCell(eS(node.disk, node.disk_total), node.disk_total > 0),
    L.jsxs("td", { className: "node-cell-traffic", children: [
      L.jsxs("div", { className: "node-traffic-heading", children: [
        L.jsx("span", { children: hasTraffic ? eO(traffic) : "—" }),
        hasTraffic && hasQuota ? L.jsx("span", { className: "node-traffic-percent",
          children: `${trafficPercent.toFixed(1)}%` }) : null,
      ] }),
      L.jsx("div", { className: "node-traffic-quota", children: hasQuota ? `/ ${eO(node.traffic_limit)}` : "/ ∞" }),
      hasTraffic && hasQuota ? L.jsx("span", { className: "node-meter", "aria-hidden": true, children:
        L.jsx("span", { style: { width: `${trafficPercent}%` } }),
      }) : null,
    ] }),
    ...(pingEnabled ? [
      L.jsx("td", { className: "node-probes", children: probeValues("latencyDisplay") }),
      L.jsx("td", { className: "node-probes", children: probeValues("lossDisplay") }),
    ] : []),
  ] });
}
