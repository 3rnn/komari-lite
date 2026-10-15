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
            onClick: toggleView, children: view === "list" ? "Card view" : "List view",
          }),
        ],
      }),
      L.jsx(rz, {}),
    ],
  });
}
