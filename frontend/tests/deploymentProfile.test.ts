import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const source = fs.readFileSync(
  path.join(root, "src/pages/admin/index.tsx"),
  "utf8",
);
const nodeContextSource = fs.readFileSync(
  path.join(root, "src/contexts/NodeDetailsContext.tsx"),
  "utf8",
);
const globalCssSource = fs.readFileSync(
  path.join(root, "src/global.css"),
  "utf8",
);
const installers = ["install.sh", "install.ps1"].map((name) => fs.readFileSync(
  path.resolve(root, "../backend/web/api/public/agent_installers", name),
  "utf8",
));

test("deployment settings are restored and saved per node", () => {
  assert.match(source, /client\/\$\{node\.uuid\}\/deployment-profile/);
  assert.match(source, /cache: "no-store"/);
  assert.match(source, /body: JSON\.stringify\(\{ profile: deploymentProfile\(\) \}\)/);
});

test("one-click Agent command uses the pinned GitHub Release installers and binaries", () => {
  assert.match(source, /--install-source/);
  assert.match(source, /const agentReleaseVersion = "1\.0\.26";/);
  assert.match(source, /const agentReleaseSource = `https:\/\/github\.com\/3rnn\/komari-lite\/releases\/download\/v\$\{agentReleaseVersion\}`;/);
  assert.equal((source.match(/"--install-source", agentReleaseSource/g) ?? []).length, 1);
  assert.equal((source.match(/selectedPlatform === "windows"\s*\? `\$\{agentReleaseSource\}\/install\.ps1`\s*:\s*`\$\{agentReleaseSource\}\/install\.sh`/g) ?? []).length, 1);
  assert.doesNotMatch(source, /panelAgentDistribution\(host\)|\/releases\/latest|\/agent\/download/);
});

test("unsupported macOS one-click command is not offered and command history is warned about", () => {
  assert.doesNotMatch(source, /value="macos"|case "macos"|bash <\(curl|zsh <\(curl/);
  assert.match(source, /shell history/i);
  assert.match(source, /token/i);
});

test("GitHub installers verify their Agent binary before changing an existing service", () => {
  for (const installer of installers) {
    assert.match(installer, /SHA256SUMS\.txt/);
    assert.match(installer, /checksum mismatch/i);
    const verification = installer.search(/checksum mismatch/i);
    const mutation = installer.includes("function Restore-Previous")
      ? installer.indexOf('Log-Step "Configuring Windows service', verification)
      : installer.indexOf('systemctl stop "${service_name}.service" || { rollback_install;', verification);
    assert.ok(mutation > verification, "service mutation must follow checksum verification");
  }
});

test("one-click deployment removes installation settings and uses safe defaults", () => {
  assert.doesNotMatch(source, /admin\.nodeTable\.installationSettings/);
  assert.doesNotMatch(source, /admin\.nodeTable\.reinstallRequired/);
  assert.match(source, /admin\.nodeTable\.onlineCollectionSettings/);
  assert.match(source, /args = \["-e", host, "-t", token, "--disable-web-ssh", "--disable-auto-update"/);
  assert.doesNotMatch(source, /args\.push\("--ignore-unsafe-cert"\)/);
  assert.match(
    source,
    /<Button\s+mt="2"\s+variant="solid"[\s\S]*?admin\.nodeTable\.saveAndDispatch/,
  );
});

test("deployment UI shows only the current delivery state", () => {
  assert.match(source, /admin-deployment-delivery/);
  assert.match(source, /deliveryStatusTitle/);
  assert.match(source, /deliveryNotStarted/);
  assert.match(source, /admin-deployment-delivery-current/);
  assert.match(source, /deliveryPresentation\.label/);
  assert.match(source, /deliveryPresentation\.hint/);
  assert.match(source, /aria-live="polite"/);
  assert.doesNotMatch(source, /deliverySteps\.map/);
  assert.doesNotMatch(source, /deliveryRevision[\s\S]*revision: deliveryState\.revision/);
});

test("server Agent column shows the matching delivery state below the version", () => {
  assert.match(source, /node\.deployment_status/);
  assert.match(source, /admin-agent-config-status/);
  assert.match(source, /admin\.nodeTable\.deliverySaved/);
  assert.match(source, /admin\.nodeTable\.deliverySent/);
  assert.match(source, /admin\.nodeTable\.deliveryApplied/);
  assert.match(source, /admin\.nodeTable\.deliveryFailed/);
  assert.match(nodeContextSource, /hydrateLegacyDeploymentStatuses/);
  assert.match(nodeContextSource, /Object\.hasOwn\(node, "deployment_status"\)/);
});

test("mobile Agent version and delivery state align left without changing desktop alignment", () => {
  assert.match(source, /className="admin-node-agent-cell[^"]*items-center[^"]*text-center/);
  assert.match(
    globalCssSource,
    /@media \(max-width: 767px\)[\s\S]*\.admin-node-table \.admin-node-agent-cell \{[\s\S]*align-items: flex-start !important;[\s\S]*text-align: left !important;/,
  );
});

test("deployment actions keep stable button content while a request is pending", () => {
  assert.match(source, /profileAction, setProfileAction/);
  assert.match(source, /aria-busy=\{profileAction === "dispatch"\}/);
  assert.match(source, /aria-busy=\{profileAction === "copy"\}/);
  assert.doesNotMatch(source, /loading=\{profileAction === "(?:dispatch|copy)"\}/);
  assert.doesNotMatch(source, /profileAction === "(?:dispatch|copy)" \|\|/);
  assert.doesNotMatch(source, /aria-disabled=\{Boolean\(profileAction\)\}/);
  assert.doesNotMatch(source, /savingProfile/);
});

test("Agent command copy uses the Edge-compatible clipboard fallback", () => {
  assert.match(source, /import \{ writeClipboardText \} from "@\/utils\/clipboard"/);
  const copyStart = source.indexOf("writeClipboardText(generateCommand())");
  const saveStart = source.indexOf("const response = await adminNodeRequest(", copyStart);
  assert.ok(copyStart >= 0 && saveStart > copyStart);
  assert.match(source, /\(value\) => \(\{ ok: true as const, value \}\)/);
  assert.match(source, /copyResult\.value\.confirmed/);
  assert.match(source, /installCommandCopyDenied/);
  assert.match(source, /installCommandCopyUnconfirmed/);
  assert.doesNotMatch(source, /navigator\.clipboard\.writeText\(generateCommand\(\)\)/);
});

test("mobile deployment copy shows an inline confirmed or failed result", () => {
  assert.match(source, /const isMobile = useIsMobile\(\)/);
  assert.match(source, /copyFeedback, setCopyFeedback/);
  assert.match(source, /isMobile && copyFeedback/);
  assert.match(source, /copyFeedback\.kind === "success"/);
  assert.match(source, /role="status"/);
  assert.match(source, /aria-live="polite"/);
  assert.match(source, /commandTextAreaRef\.current\?\.select\(\)/);
});


test("servers page routes every admin GET through the session-aware request", () => {
  assert.doesNotMatch(source, /\bfetch\s*\(/);
  assert.equal((source.match(/adminNodeRequest\(`\/api\/admin\/client\/\$\{node\.uuid\}\/deployment-profile`/g) ?? []).length, 2);
  assert.match(source, /adminNodeRequest\(`\/api\/admin\/client\/\$\{node\.uuid\}\/traffic-calibration`/);
});

test("unsupported auto-discovery installation is not offered or generated", () => {
  assert.doesNotMatch(source, /AutoDiscoverySection|--auto-discovery|auto_discovery_key/);
});

test("Windows generated install command uses encoded PowerShell rather than cmd-quoted program text", () => {
  assert.match(source, /windowsInstallCommand\(scriptUrl, args\)/);
  assert.doesNotMatch(source, /-Command `|pwsh\.exe -NoProfile -ExecutionPolicy Bypass -Command /);
});
