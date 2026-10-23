// Input is the existing public node context; never fetch a second node list.
export function selectGlassTopThroughput(nodes, now = Date.now()) {
  let top = null;
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
    if (!top || speed > top.speed || (speed === top.speed && node.uuid < top.uuid)) {
      top = { uuid: node.uuid, name: node.name, speed, expiresAt: time + 60_001 };
    }
  }
  return top;
}

function glassNetworkStat({ nodes, ...props }) {
  const [, refreshExpiry] = (0,R.useState)(0);
  const top = selectGlassTopThroughput(nodes);
  const expiresAt = top?.expiresAt;
  // A single local deadline clears frozen data if the existing feed stops.
  // This never fetches, opens a subscription, or polls an empty state.
  (0,R.useEffect)(() => {
    if (expiresAt === undefined) return;
    const timer = setTimeout(() => refreshExpiry(value => value + 1), Math.max(1, expiresAt - Date.now()));
    return () => clearTimeout(timer);
  }, [expiresAt]);
  const host = top?.name ?? "—";
  const hostSpeed = top ? ej(top.speed) : "—";
  // Reuse the original cell and its sizing content. Only overlay the first two
  // rows; this preserves the original grid geometry, even at narrow widths.
  const cell = rK(props);
  const [heading, value, footer] = cell.props.children;
  const sizing = row => R.cloneElement(row, { "aria-hidden": true, style: { visibility: "hidden" } });
  const header = R.cloneElement(heading, { className: `${heading.props.className} overview-network-heading` },
    ...heading.props.children.map(sizing),
    (0,L.jsxs)("span", { className: "overview-network-heading-content", children: [
      (0,L.jsx)("span", { className: "overview-network-label", title: props.label, children: props.label }),
      (0,L.jsx)("span", { className: "overview-network-host", title: top ? `Highest live throughput: ${host}` : "No fresh online sample", children: host }),
    ] }));
  const main = R.cloneElement(value, { className: `${value.props.className} overview-network-value` },
    ...value.props.children.filter(Boolean).map(sizing),
    (0,L.jsxs)("span", { className: "overview-network-value-content", style: { "--network-total-chars": String(props.value).length, "--network-host-chars": hostSpeed.length }, children: [
      (0,L.jsx)("span", { className: "overview-network-total", title: props.value, children: props.value }),
      (0,L.jsx)("span", { className: "overview-network-host-speed", title: top ? `${host}: ${hostSpeed}` : "No fresh online sample", children: hostSpeed }),
    ] }));
  return R.cloneElement(cell, { className: `${cell.props.className} overview-network-stat`, "data-network-empty": !top }, header, main, footer);
}
