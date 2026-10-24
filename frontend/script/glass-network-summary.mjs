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

// At most one real observation per second; no backfill or timer sampling.
export function observeGlassThroughput(history, nodes, now = Date.now(), visible = true) {
  const retained = history.filter(point => point.time > now - 60_000 && point.time <= now);
  const ranked = visible ? rankGlassThroughput(nodes, now) : [];
  const signature = ranked.map(node => `${node.uuid}:${node.expiresAt}:${node.speed}`).sort().join("|");
  const previous = retained.at(-1);
  if (previous?.signature === signature || (signature && previous?.observedSignature === signature) || (!previous && !signature)) return retained;
  const sum = ranked.reduce((total, node) => total + node.speed, 0);
  const sameBucket = previous && Math.floor(previous.time / 1000) === Math.floor(now / 1000);
  const point = { time: now, value: signature && Number.isFinite(sum) ? sum : null, signature,
    observedSignature: signature || previous?.observedSignature || "",
    breakBefore: previous?.value === null || Boolean(sameBucket && previous?.breakBefore) };
  if (sameBucket) retained.pop();
  return [...retained, point].slice(-60);
}

export function glassThroughputPaths(history, now = Date.now()) {
  const points = history.filter(point => point.time > now - 60_000 && point.time <= now);
  const maximum = Math.max(1, ...points.map(point => point.value ?? 0));
  const paths = [];
  let previous;
  for (const point of points) {
    if (point.value === null) { previous = undefined; continue; }
    const x = ((point.time - (now - 60_000)) / 60_000 * 120).toFixed(2);
    const y = (35 - point.value / maximum * 30).toFixed(2);
    // Feed silence is a gap, not an interpolated throughput claim.
    if (!previous || point.breakBefore || point.time - previous.time > 2500) paths.push(`M${x},${y}l0,0`);
    else paths[paths.length - 1] += `L${x},${y}`;
    previous = point;
  }
  return paths;
}

function glassTrafficStat(props) {
  const cell = rK(props);
  return R.cloneElement(cell, { className: `${cell.props.className} overview-traffic-stat` });
}

function glassNetworkStat({ nodes, ...props }) {
  const [history, setHistory] = (0,R.useState)([]);
  // This hook consumes only existing feed renders. Local expiry never samples.
  (0,R.useEffect)(() => {
    setHistory(previous => observeGlassThroughput(previous, nodes, Date.now(), typeof document === "undefined" || !document.hidden));
  }, [nodes]);
  (0,R.useEffect)(() => {
    if (typeof document === "undefined") return;
    const onVisibility = () => {
      // Neither hiding nor resuming may sample a cached feed snapshot.
      setHistory(previous => observeGlassThroughput(previous, [], Date.now(), false));
    };
    document.addEventListener("visibilitychange", onVisibility);
    return () => document.removeEventListener("visibilitychange", onVisibility);
  }, []);
  const chartPaths = glassThroughputPaths(history);
  const chart = (0,L.jsxs)("div", { className: "overview-network-chart", "data-sample-count": history.filter(point => point.value !== null && point.time > Date.now() - 60_000).length, children: [
    (0,L.jsx)("span", { className: "overview-network-live", children: rankGlassThroughput(nodes).length ? "LIVE" : "NO DATA" }),
    chartPaths.length ? (0,L.jsx)("svg", { viewBox: "0 0 120 40", role: "img", "aria-label": "Observed aggregate upload plus download throughput over the last 60 seconds; gaps indicate missing observations.", preserveAspectRatio: "none", children: chartPaths.map((d, index) => (0,L.jsx)("path", { d, fill: "none", stroke: "currentColor", strokeWidth: 1.5, strokeLinecap: "round", vectorEffect: "non-scaling-stroke" }, index)) }) : (0,L.jsx)("span", { className: "overview-network-chart-empty", children: "Awaiting live samples" }),
    (0,L.jsxs)("span", { className: "overview-network-axis", "aria-hidden": true, children: [(0,L.jsx)("span", { children: "−60s" }), (0,L.jsx)("span", { children: "Now" })] }),
  ] });
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
          (0,L.jsx)("span", { className: "overview-network-host-speed", "data-short-speed": speed.replace(/([KMGTP])B\/s$/, "$1/s"), children: speed }),
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
  return R.cloneElement(cell, { className: `${cell.props.className} overview-network-stat`, "data-network-empty": !top, style: { "--network-total-chars": String(props.value).length, "--network-host-chars": top ? ej(top.speed).length : 1 } }, header, main, footer, chart, (0,L.jsxs)("div", { className: "overview-network-desktop overview-network-desktop-header", children: [(0,L.jsx)("span", { children: "TOP 5" }), (0,L.jsx)("span", { children: ranked.length ? "LIVE" : "NO DATA" })] }), (0,L.jsx)("div", { className: "overview-network-desktop overview-network-desktop-rates", children: footer.props.children }), (0,L.jsxs)("div", { className: "overview-network-rank-heading", children: [(0,L.jsx)("span", { children: "TOP THROUGHPUT" }), (0,L.jsx)("span", { children: ranked.length > 4 ? `${ranked.length} nodes · scroll for more` : "Upload + download" })] }));
}
