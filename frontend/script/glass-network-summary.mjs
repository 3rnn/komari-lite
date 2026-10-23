// Input is the existing public node context; never fetch a second node list.
export function selectGlassTopThroughput(nodes, now = Date.now()) {
  return rankGlassThroughput(nodes, now)[0] ?? null;
}

export function rankGlassThroughput(nodes, now = Date.now()) {
  const ranked = [];
  for (const node of nodes ?? []) {
    // A 60-second presentation window, independent of server online detection.
    // The context contains only publicly exposed nodes; hidden is defensive.
    const time = Date.parse(node.updated_at_live);
    if (node.online !== true || node.hidden || typeof node.name !== "string" || !node.name.trim()
      || typeof node.uuid !== "string" || !node.uuid || !Number.isFinite(time)
      || time > now || now - time > 60_000
      || !Number.isFinite(node.net_out) || node.net_out < 0
      || !Number.isFinite(node.net_in) || node.net_in < 0) continue;
    const speed = node.net_out + node.net_in;
    if (!Number.isFinite(speed)) continue;
    ranked.push({ uuid: node.uuid, name: node.name, speed, expiresAt: time + 60_001 });
  }
  return ranked.sort((a, b) => b.speed - a.speed || (a.uuid < b.uuid ? -1 : a.uuid > b.uuid ? 1 : 0));
}

function glassNetworkStat({ nodes, ...props }) {
  const [, refreshExpiry] = (0,R.useState)(0);
  const ranked = rankGlassThroughput(nodes);
  const top = ranked[0];
  const expiresAt = ranked.length ? Math.min(...ranked.map(node => node.expiresAt)) : undefined;
  // A single local deadline clears frozen data if the existing feed stops.
  // This never fetches, opens a subscription, or polls an empty state.
  (0,R.useEffect)(() => {
    if (expiresAt === undefined) return;
    const timer = setTimeout(() => refreshExpiry(value => value + 1), Math.max(1, expiresAt - Date.now()));
    return () => clearTimeout(timer);
  }, [expiresAt]);
  const rail = (0,L.jsx)("ol", {
    className: "overview-network-ranking", tabIndex: 0,
    "aria-label": "Live throughput ranking, upload plus download. Scroll for more nodes.",
    children: ranked.length ? ranked.map((node, index) => {
      const speed = ej(node.speed);
      return (0,L.jsxs)("li", {
        className: "overview-network-rank-row", title: `${index + 1}. ${node.name}: ${speed}`,
        "aria-label": `Rank ${index + 1}, ${node.name}, ${speed}`,
        children: [
          (0,L.jsx)("span", { className: "overview-network-rank", "aria-hidden": true, children: String(index + 1).padStart(2, "0") }),
          (0,L.jsx)("span", { className: "overview-network-host", children: node.name }),
          (0,L.jsx)("span", { className: "overview-network-host-speed", children: speed }),
        ],
      }, node.uuid);
    }) : (0,L.jsx)("li", { className: "overview-network-empty", children: "No fresh online sample" }),
  });
  // The original three sizing rows remain in flow. One bounded rail is anchored
  // in the heading's right annotation area, never in the other metrics.
  const cell = rK(props);
  const [heading, value, footer] = cell.props.children;
  const sizing = row => R.cloneElement(row, { "aria-hidden": true, style: { visibility: "hidden" } });
  const header = R.cloneElement(heading, { className: `${heading.props.className} overview-network-heading` },
    ...heading.props.children.map(sizing),
    (0,L.jsx)("span", { className: "overview-network-heading-content", children:
      (0,L.jsx)("span", { className: "overview-network-label", title: props.label, children: props.label }),
    }), rail);
  const main = R.cloneElement(value, { className: `${value.props.className} overview-network-value` },
    ...value.props.children.filter(Boolean).map(sizing),
    (0,L.jsx)("span", { className: "overview-network-value-content", children:
      (0,L.jsx)("span", { className: "overview-network-total", title: props.value, children: props.value }),
    }));
  return R.cloneElement(cell, { className: `${cell.props.className} overview-network-stat`, "data-network-empty": !top, style: { "--network-total-chars": String(props.value).length, "--network-host-chars": top ? ej(top.speed).length : 1 } }, header, main, footer);
}
