// Injected into the pinned Glass module: R and L are its React/runtime imports.
function rW() {
  const [view, setView] = R.useState("cards");
  R.useEffect(() => {
    try {
      if (localStorage.getItem("komari-glass-node-view") === "list") setView("list");
    } catch { /* Storage can be disabled; the view remains usable. */ }
  }, []);
  function toggleView() {
    const next = view === "list" ? "cards" : "list";
    setView(next);
    try { localStorage.setItem("komari-glass-node-view", next); } catch { /* Optional persistence. */ }
  }
  return L.jsxs("div", {
    className: "space-y-3.5 sm:space-y-4 glass-node-home",
    "data-node-view": view,
    children: [
      L.jsx(rH, {}),
      L.jsxs("div", {
        className: "node-view-toolbar",
        children: [
          L.jsx("span", { className: "node-view-heading", children: "Nodes" }),
          L.jsx("button", {
            type: "button", className: "node-view-toggle",
            "aria-label": "List view", "aria-pressed": view === "list",
            title: view === "list" ? "Card view" : "List view",
            onClick: toggleView, children: L.jsx("svg", {
              width: 18, height: 18, viewBox: "0 0 24 24", fill: "none",
              stroke: "currentColor", strokeWidth: 1.7, strokeLinecap: "round",
              "aria-hidden": true, focusable: "false",
              children: L.jsx("path", { d: view === "list"
                ? "M3 3h7v7H3z M14 3h7v7h-7z M3 14h7v7H3z M14 14h7v7h-7z"
                : "M8 6h13 M8 12h13 M8 18h13 M3 6h1 M3 12h1 M3 18h1" }),
            }),
          }),
        ],
      }),
      L.jsx(rz, {}),
    ],
  });
}
