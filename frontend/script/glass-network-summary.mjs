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
    ranked.push({ uuid: node.uuid, name: node.name, speed, upload: node.net_out, download: node.net_in, expiresAt: time + 60_001 });
  }
  return ranked.sort((a, b) => b.speed - a.speed || (a.uuid < b.uuid ? -1 : a.uuid > b.uuid ? 1 : 0));
}

// At most one real observation per second; no backfill or timer sampling.
export function observeGlassThroughput(history, nodes, now = Date.now(), visible = true) {
  const retained = history.filter(point => point.time > now - 60_000 && point.time <= now);
  const ranked = visible ? rankGlassThroughput(nodes, now) : [];
  const signature = ranked.map(node => `${node.uuid}:${node.expiresAt}:${node.upload}:${node.download}`).sort().join("|");
  const previous = retained.at(-1);
  if (previous?.signature === signature || (signature && previous?.observedSignature === signature) || (!previous && !signature)) return retained;
  const upload = ranked.reduce((total, node) => total + node.upload, 0);
  const download = ranked.reduce((total, node) => total + node.download, 0);
  const sum = upload + download;
  const sameBucket = previous && Math.floor(previous.time / 1000) === Math.floor(now / 1000);
  const hasSample = Boolean(signature) && Number.isFinite(sum);
  const point = { time: now, value: hasSample ? sum : null, upload: hasSample ? upload : null,
    download: hasSample ? download : null, signature,
    observedSignature: signature || previous?.observedSignature || "",
    breakBefore: previous?.value === null || Boolean(sameBucket && previous?.breakBefore) };
  if (sameBucket) retained.pop();
  return [...retained, point].slice(-60);
}

export function glassThroughputPaths(history, now = Date.now(), channel = "value", maximum) {
  const points = history.filter(point => point.time > now - 60_000 && point.time <= now);
  const scale = maximum ?? Math.max(1, ...points.map(point => point[channel] ?? 0));
  const paths = [];
  let previous;
  for (const point of points) {
    if (!Number.isFinite(point[channel])) { previous = undefined; continue; }
    const x = ((point.time - (now - 60_000)) / 60_000 * 120).toFixed(2);
    const y = (35 - point[channel] / scale * 30).toFixed(2);
    // Feed silence is a gap, not an interpolated throughput claim.
    if (!previous || point.breakBefore || point.time - previous.time > 2500) paths.push(`M${x},${y}l0,0`);
    else paths[paths.length - 1] += `L${x},${y}`;
    previous = point;
  }
  return paths;
}

function rH() {
  const { overview, settings, nodes, feedLive } = eB();
  const stats = settings.showStatsBar ? [
    ...(settings.showOnline ? [["Nodes online", `${overview.online} / ${overview.total}`]] : []),
    ...(settings.showSpeed ? [
      ["Upload", feedLive ? ej(overview.speedUp) : "—"],
      ["Download", feedLive ? ej(overview.speedDown) : "—"],
    ] : []),
    ...(settings.showTraffic ? [["Total traffic", eO(overview.trafficTotal)]] : []),
  ] : [];
  return L.jsxs("section", { className: "glass-infrastructure", "aria-labelledby": "glass-infrastructure-title", children: [
    L.jsxs("div", { className: "glass-overview-heading", children: [
      L.jsx("h1", { id: "glass-infrastructure-title", className: "glass-section-title", children: "Infrastructure" }),
      L.jsxs("div", { className: "glass-raid", role: "status", "data-live": feedLive === true, children: [
        L.jsx("span", { className: "glass-raid-icon", "aria-hidden": true }),
        L.jsx("span", { className: "glass-raid-label", children: "PARA-RAID" }),
        L.jsx("strong", { children: feedLive === true ? "Live" : feedLive === false ? "Disconnected" : "Connecting" }),
      ] }),
    ] }),
    stats.length ? L.jsx("dl", { className: "glass-overview-stats", children: stats.map(([label, value]) =>
      L.jsxs("div", { children: [L.jsx("dt", { children: label }), L.jsx("dd", { children: value })] }, label)),
    }) : null,
    settings.showStatsBar && settings.showAssets ? L.jsxs("p", { className: "glass-overview-assets", children: [
      L.jsx("span", { children: `Active assets ${ex(overview.totalValue, overview.currency)}` }),
      L.jsx("span", { children: `Remaining ${ex(overview.remainingValue, overview.currency)}` }),
      L.jsx("span", { children: `${overview.paidCount} billed` }),
    ] }) : null,
    settings.showStatsBar && settings.showSpeed ? L.jsx(GlassNetworkPanels, { nodes, feedLive }) : null,
  ] });
}

function GlassNetworkPanels({ nodes, feedLive }) {
  const [history, setHistory] = R.useState([]);
  const [, refreshExpiry] = R.useState(0);
  R.useEffect(() => {
    const visible = typeof document === "undefined" || !document.hidden;
    setHistory(previous => observeGlassThroughput(previous, nodes, Date.now(), feedLive === true && visible));
  }, [nodes, feedLive]);
  R.useEffect(() => {
    if (typeof document === "undefined") return;
    // Resuming a background tab must wait for a new feed observation.
    const onVisibility = () => setHistory(previous => observeGlassThroughput(previous, [], Date.now(), false));
    document.addEventListener("visibilitychange", onVisibility);
    return () => document.removeEventListener("visibilitychange", onVisibility);
  }, []);
  const now = Date.now();
  const ranked = feedLive === true ? rankGlassThroughput(nodes, now) : [];
  const observations = history.filter(point => point.time > now - 60_000 && point.time <= now);
  const expiresAt = Math.min(Infinity, ...ranked.map(node => node.expiresAt),
    ...observations.map(point => point.time + 60_001));
  R.useEffect(() => {
    if (!Number.isFinite(expiresAt)) return;
    const timer = setTimeout(() => refreshExpiry(value => value + 1), Math.max(1, expiresAt - Date.now()));
    return () => clearTimeout(timer);
  }, [expiresAt]);
  const scale = Math.max(1, ...observations.flatMap(point => [point.upload ?? 0, point.download ?? 0]));
  const traces = [
    ["upload", glassThroughputPaths(observations, now, "upload", scale)],
    ["download", glassThroughputPaths(observations, now, "download", scale)],
  ];
  const hasSamples = observations.some(point => point.value !== null);
  const highest = ranked[0]?.speed || 1;

  return L.jsxs("div", { className: "glass-network-panels", children: [
    L.jsxs("section", { className: "glass-instrument glass-throughput", "aria-labelledby": "glass-throughput-title", children: [
      L.jsxs("div", { className: "glass-throughput-heading", children: [
        L.jsx("h2", { id: "glass-throughput-title", children: "Realtime throughput" }),
        L.jsxs("div", { className: "glass-throughput-key", children: [
          L.jsx("span", { className: "glass-trace-upload", children: "Upload" }),
          L.jsx("span", { className: "glass-trace-download", children: "Download" }),
        ] }),
      ] }),
      L.jsxs("div", { className: "glass-throughput-plot", "data-sample-count": observations.filter(point => point.value !== null).length, children: [
        L.jsx("div", { className: "glass-throughput-scale", "aria-hidden": true,
          children: [scale, scale / 2, 0].map((value, index) => L.jsx("span", { children: hasSamples ? ej(value, 1) : "" }, index)),
        }),
        hasSamples ? L.jsx("svg", {
          viewBox: "0 5 120 30", preserveAspectRatio: "none", role: "img",
          "aria-label": "Observed upload and download over the last 60 seconds",
          children: traces.flatMap(([channel, paths]) => paths.map((path, index) => L.jsx("path", {
            d: path, className: `glass-trace-${channel}`, vectorEffect: "non-scaling-stroke",
          }, `${channel}-${index}`))),
        }) : L.jsx("p", { className: "glass-throughput-empty", children: "Awaiting live samples" }),
        L.jsxs("div", { className: "glass-throughput-time", "aria-hidden": true,
          children: [L.jsx("span", { children: "60s ago" }), L.jsx("span", { children: "Now" })],
        }),
      ] }),
    ] }),
    L.jsxs("section", { className: "glass-instrument glass-ranking", "aria-labelledby": "glass-ranking-title", children: [
      L.jsx("h2", { id: "glass-ranking-title", children: "Top throughput" }),
      ranked.length ? L.jsx("ol", {
        className: "overview-network-ranking", tabIndex: 0, "aria-label": "Live throughput ranking, upload plus download",
        children: ranked.map((node, index) => L.jsxs("li", {
          className: "overview-network-rank-row", title: `${index + 1}. ${node.name}: ${ej(node.speed)}`,
          children: [
            L.jsx("span", { className: "overview-network-host", children: node.name }),
            L.jsx("span", { className: "node-meter", "aria-hidden": true, children:
              L.jsx("span", { style: { width: `${node.speed / highest * 100}%` } }),
            }),
            L.jsx("span", { className: "overview-network-host-speed", children: ej(node.speed) }),
          ],
        }, node.uuid)),
      }) : L.jsx("p", { className: "overview-network-empty", children: "No fresh online sample" }),
    ] }),
  ] });
}
