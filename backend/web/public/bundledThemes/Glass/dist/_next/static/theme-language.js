/* Glass public-site language bridge: shares the admin's `language` localStorage/cookie. */
(() => {
  "use strict";
  const STORAGE_KEY = "language";
  const COOKIE_KEY = "language";
  const TOGGLE_ID = "komari-public-language-toggle";
  let scheduled = false;

  const exact = {
    "在线节点": "Online nodes",
    "全部运行正常": "All systems operational",
    "在线资产": "Active assets",
    "累计流量": "Total traffic",
    "实时网速": "Live network speed",
    "上传": "Upload",
    "下载": "Download",
    "在线": "Online",
    "离线": "Offline",
    "CPU": "CPU",
    "内存": "Memory",
    "硬盘": "Disk",
    "流量": "Traffic",
    "延迟": "Latency",
    "丢包": "Packet loss",
    "切换到深色主题": "Switch to dark theme",
    "切换到浅色主题": "Switch to light theme",
    "进入后台": "Open admin",
    "剩余": "Remaining",
    "台计费": "billed",
    "天": "days",
  };

  function readCookie(name) {
    const prefix = `${name}=`;
    for (const item of document.cookie.split(";")) {
      const value = item.trim();
      if (value.startsWith(prefix)) return decodeURIComponent(value.slice(prefix.length));
    }
    return "";
  }

  function language() {
    const raw = readCookie(COOKIE_KEY) || localStorage.getItem(STORAGE_KEY) || "zh-CN";
    return /^en(?:-|$)/i.test(String(raw).replace(/_/g, "-")) ? "en" : "zh";
  }

  function translate(value) {
    const text = value.trim();
    let output = exact[text];
    if (output) return value.replace(text, output);
    output = text
      .replace(/^剩余\s+/, "Remaining ")
      .replace(/^全部运行正常$/, "All systems operational")
      .replace(/^(\d+)\s*台离线$/, "$1 offline")
      .replace(/^(\d+)\s*台计费$/, "$1 billed")
      .replace(/^在线\s*(\d+)\s*天$/, "Online $1 days")
      .replace(/^(\d+)\s*天$/, "$1 days")
      .replace(/\/\s*年$/, "/ year")
      .replace(/\/\s*(\d+)\s*台$/, "/ $1 nodes");
    return output === text ? value : value.replace(text, output);
  }

  function translateTree() {
    if (language() !== "en" || !document.body) return;
    const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
    const nodes = [];
    while (walker.nextNode()) nodes.push(walker.currentNode);
    for (const node of nodes) {
      if (/[\u4e00-\u9fff]/.test(node.nodeValue || "")) node.nodeValue = translate(node.nodeValue || "");
    }
    for (const element of document.querySelectorAll("[aria-label],[title]")) {
      for (const name of ["aria-label", "title"]) {
        const value = element.getAttribute(name);
        if (value && /[\u4e00-\u9fff]/.test(value)) element.setAttribute(name, translate(value));
      }
    }
  }

  function setLanguage(next) {
    const value = next === "en" ? "en-US" : "zh-CN";
    localStorage.setItem(STORAGE_KEY, value);
    document.cookie = `${COOKIE_KEY}=${encodeURIComponent(value)}; path=/; max-age=31536000; SameSite=Lax`;
    location.reload();
  }

  function installToggle() {
    const actions = document.querySelector("header .flex.shrink-0.items-center.gap-1");
    if (!actions || document.getElementById(TOGGLE_ID)) return;
    const current = language();
    const button = document.createElement("button");
    button.id = TOGGLE_ID;
    button.type = "button";
    button.className = "header-action-btn icon-btn text-[11px] font-semibold";
    button.textContent = current === "en" ? "中" : "EN";
    button.title = current === "en" ? "切换到简体中文" : "Switch to English";
    button.setAttribute("aria-label", button.title);
    button.addEventListener("click", () => setLanguage(current === "en" ? "zh" : "en"));
    actions.prepend(button);
  }

  function apply() {
    document.documentElement.lang = language() === "en" ? "en-US" : "zh-CN";
    installToggle();
    translateTree();
  }

  function schedule() {
    if (scheduled) return;
    scheduled = true;
    requestAnimationFrame(() => { scheduled = false; apply(); });
  }

  window.addEventListener("storage", (event) => {
    if (event.key === STORAGE_KEY) location.reload();
  });
  new MutationObserver(schedule).observe(document.documentElement, { childList: true, subtree: true, characterData: true });
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", apply, { once: true });
  else apply();
})();
