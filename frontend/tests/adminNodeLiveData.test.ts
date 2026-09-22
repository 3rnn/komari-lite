import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const hookSource = readFileSync("src/hooks/use-admin-node-live-data.ts", "utf8");
const pageSource = readFileSync("src/pages/admin/index.tsx", "utf8");
const pingPageSource = readFileSync("src/pages/admin/pingTask.tsx", "utf8");
const pingTaskSource = readFileSync("src/pages/admin/pingTask_Task.tsx", "utf8");
const pingServerSource = readFileSync("src/pages/admin/pingTask_Server.tsx", "utf8");
const offlineSource = readFileSync("src/pages/admin/notification/offline.tsx", "utf8");
const trafficReportSource = readFileSync("src/pages/admin/notification/traffic_report.tsx", "utf8");
const loadSource = readFileSync("src/pages/admin/notification/load.tsx", "utf8");
const pingLossSource = readFileSync("src/pages/admin/notification/ping_loss.tsx", "utf8");
const globalCssSource = readFileSync("src/global.css", "utf8");
const tableSource = readFileSync("src/components/ui/table.tsx", "utf8");
const paginationSource = readFileSync("src/components/admin/AdminPagination.tsx", "utf8");
const paginationUtilitySource = readFileSync("src/utils/adminPagination.ts", "utf8");
const paginationHookSource = readFileSync("src/hooks/useAdminDefaultPageSize.ts", "utf8");
const statusSummarySource = readFileSync("src/components/admin/AdminNodeStatusSummary.tsx", "utf8");
const selectionCountSource = readFileSync("src/components/admin/AdminSelectionCount.tsx", "utf8");
const sessionsSource = readFileSync("src/pages/admin/sessions.tsx", "utf8");
const metricsSource = readFileSync("src/pages/admin/settings/metrics.tsx", "utf8");
const logSource = readFileSync("src/pages/admin/log.tsx", "utf8");
const settingCardSource = readFileSync("src/components/admin/SettingCard.tsx", "utf8");

test("admin node status uses one guarded compact poll", () => {
  assert.match(hookSource, /ADMIN_NODE_LIVE_INTERVAL_MS = 5000/);
  assert.match(hookSource, /common:getNodesLatestStatus"\s*,\s*\{ compact: true \}/);
  assert.match(hookSource, /if \(running \|\| stopped \|\| document\.hidden\) return/);
  assert.match(hookSource, /visibilitychange/);
});

test("admin node table keeps persisted ordering and prioritizes identity and billing", () => {
  assert.match(pageSource, /\/api\/admin\/client\/order/);
  assert.doesNotMatch(pageSource, /ResourceStatus|TrafficQuota|ResourceUsage/);
  assert.match(pageSource, /t\("common\.group", "分组"\)/);
  assert.match(pageSource, /t\("common\.remark", "备注"\)/);
  assert.match(pageSource, /w-\[224px\].*admin\.nodeTable\.billing/);
  assert.match(pageSource, /nodeTable\.agent[\s\S]*publicVersion\(node\.version\)/);
  assert.match(pageSource, /admin-node-country-flag/);
  assert.match(pageSource, /reorderEnabled=\{!searchTerm\.trim\(\) && statusFilter === "all" && !routeNode && !routeAlert\}/);
  assert.doesNotMatch(pageSource, /selectedNodes|handleSelectAll|handleSelectNode/);
});

test("admin node table uses the global page size and saves the complete cross-page order", () => {
  assert.match(paginationUtilitySource, /ADMIN_LIST_PAGE_SIZE = 10/);
  assert.match(paginationUtilitySource, /return \[10, 50, 100\]/);
  assert.match(paginationHookSource, /settings\.admin_default_page_size/);
  assert.match(paginationSource, /adminPageSizeOptions\(\)/);
  assert.match(pageSource, /visibleNodes = localNodes\.slice/);
  assert.match(pageSource, /PREVIOUS_PAGE_DROP_ID/);
  assert.match(pageSource, /NEXT_PAGE_DROP_ID/);
  assert.match(pageSource, /destinationPage = visiblePage - 1/);
  assert.match(pageSource, /destinationPage = visiblePage \+ 1/);
  assert.match(pageSource, /const orderData = reorderedNodes\.reduce/);
  assert.match(pageSource, /pageSize=\{pageSize\}/);
  assert.match(pageSource, /onPageSizeChange=/);
  assert.match(pageSource, /destinationPage \* pageSize - 1/);
  assert.match(pageSource, /overflow-x-auto overflow-y-hidden/);
});

test("mobile dialogs and settings cards stay within the viewport", () => {
  assert.match(pageSource, /className="admin-install-dialog"/);
  assert.match(pageSource, /admin-install-platforms/);
  assert.match(pageSource, /admin-install-options-grid grid grid-cols-2/);
  assert.match(globalCssSource, /\.rt-BaseDialogScrollPadding \{[\s\S]*width: 100%;[\s\S]*min-width: 0/);
  assert.match(globalCssSource, /\.rt-DialogContent \{[\s\S]*width: 100% !important/);
  assert.doesNotMatch(globalCssSource, /\.rt-DialogContent \{[\s\S]*transform: translate\(-50%, -50%\) !important/);
  assert.match(globalCssSource, /\.admin-install-options-grid \{[\s\S]*grid-template-columns: minmax\(0, 1fr\)/);
  assert.match(globalCssSource, /font-size: 16px !important/);
  assert.match(settingCardSource, /setting-card-header w-full min-w-0 max-w-full/);
});

test("all admin information lists share configurable pagination", () => {
  for (const source of [
    pingTaskSource,
    pingServerSource,
    offlineSource,
    trafficReportSource,
    loadSource,
    pingLossSource,
    sessionsSource,
    metricsSource,
    logSource,
  ]) {
    assert.match(source, /AdminPagination/);
    assert.match(source, /onPageSizeChange/);
  }
  assert.match(pingTaskSource, /PREVIOUS_PAGE_DROP_ID/);
  assert.match(pingTaskSource, /NEXT_PAGE_DROP_ID/);
  assert.match(pingTaskSource, /destinationPage = page - 1/);
  assert.match(pingTaskSource, /destinationPage = page \+ 1/);
});

// 主题市场与其源文件已随精简版删除，相关的源码断言不再适用。

test("admin node toolbar aligns status left and search actions right", () => {
  assert.match(pageSource, /<AdminNodeStatusSummary/);
  assert.match(statusSummarySource, /aria-pressed=\{value === filter\}/);
  assert.match(statusSummarySource, /useReduceMotionPreference\(\)/);
  assert.doesNotMatch(statusSummarySource, /useSettings\(\)/);
  assert.match(statusSummarySource, /layoutId=\{reduceMotion \? undefined : "admin-node-status-highlight"\}/);
  assert.match(statusSummarySource, /whileTap=\{reduceMotion \? undefined/);
  assert.match(statusSummarySource, /onClick=\{\(\) => onValueChange\(filter\)\}/);
  assert.match(pageSource, /flex flex-col gap-3 md:flex-row md:items-end md:justify-between/);
  assert.match(pageSource, /showStatusSummary[\s\S]*md:ml-auto md:w-auto/);
  assert.match(statusSummarySource, /flex h-10 items-center justify-center/);
  assert.doesNotMatch(pageSource, /style=\{\{ height: "48px" \}\}/);
  assert.doesNotMatch(pageSource, /lastReportRecent|liveRefreshInterval/);
  assert.doesNotMatch(pageSource, /resourceFromLatestReport/);
  assert.match(pageSource, /networkAddresses\.length > 0 \? networkAddresses\.map/);
  assert.match(pageSource, /type === "IPv6" \? compactIPv6\(address\) : address/);
  assert.match(pageSource, /flex min-w-0 flex-col justify-center text-sm leading-\[1\.125rem\] text-muted-foreground/);
  assert.match(statusSummarySource, /bg-\[var\(--color-panel-solid\)\]/);
  assert.doesNotMatch(pageSource, /md:inline-flex md:w-fit/);
});

test("server details use a centered read-only form dialog", () => {
  assert.match(pageSource, /function ReadOnlyDetailField/);
  assert.match(pageSource, /admin\.nodeDetail\.network/);
  assert.match(pageSource, /admin\.nodeDetail\.system/);
  assert.match(pageSource, /admin\.nodeDetail\.resources/);
  assert.match(pageSource, /admin\.nodeDetail\.identity/);
  assert.match(pageSource, /<AppDialogContent[\s\S]{0,500}maxWidth="720px"/);
  assert.match(pageSource, /onOpenAutoFocus=\{\(event\) => \{[\s\S]*preventDefault\(\)[\s\S]*focus\(\{ preventScroll: true \}\)/);
  assert.match(pageSource, /readOnly/);
  assert.match(pageSource, /<Button variant="soft">\{t\("admin\.nodeDetail\.done"/);
  assert.doesNotMatch(pageSource, /<Drawer/);
  assert.match(pageSource, /admin-node-detail-country-flag/);
  assert.doesNotMatch(pageSource, /admin-node-detail-country-flag[^\n]*border/);
  assert.match(globalCssSource, /\.admin-node-detail-country-flag > span \{[\s\S]*width: 36px[\s\S]*height: 27px/);
});

test("delete confirmation names the server before the question", () => {
  assert.match(pageSource, /<Dialog\.Description>[\s\S]*<Text as="span" weight="bold">\{node\.name\}<\/Text>[\s\S]*confirmDeleteQuestion/);
});

test("admin tables share one header color and mobile actions stay compact", () => {
  assert.match(tableSource, /admin-table-header/);
  assert.match(globalCssSource, /\.admin-table-header/);
  assert.match(globalCssSource, /\.admin-card-actions \{[\s\S]*justify-content: flex-start/);
  assert.match(globalCssSource, /\.admin-card-actions button \{[\s\S]*background-color: transparent !important/);
  assert.match(globalCssSource, /\.admin-card-actions button:hover \{[\s\S]*background-color: var\(--accent-a3\)/);
  assert.match(globalCssSource, /\.admin-node-table tbody td:first-child[\s\S]*position: absolute/);
  assert.match(globalCssSource, /\.admin-node-actions[\s\S]*grid-template-columns: repeat\(7/);
  assert.match(globalCssSource, /\.admin-node-country-flag > span \{[\s\S]*width: 28px[\s\S]*height: 21px/);
  assert.match(globalCssSource, /\.admin-node-actions > button \{[\s\S]*min-width: 2rem[\s\S]*min-height: 2rem/);
  assert.match(globalCssSource, /\.admin-sortable-table tbody td:first-child[\s\S]*position: absolute/);
  assert.match(globalCssSource, /\.admin-card-actions[\s\S]*grid-template-columns: repeat\(auto-fit, minmax\(2\.25rem, 2\.75rem\)\)/);
  assert.match(globalCssSource, /\.admin-responsive-table tbody td:last-child > \.admin-card-actions[\s\S]*justify-content: start/);
  assert.match(globalCssSource, /\.admin-card-actions\.admin-single-text-action[\s\S]*display: flex/);
  assert.match(globalCssSource, /\.admin-card-actions\.admin-single-text-action \{[\s\S]*justify-content: center/);
  assert.match(globalCssSource, /\.admin-single-action-label \{[\s\S]*display: none/);
  assert.match(globalCssSource, /@media \(max-width: 767px\)[\s\S]*\.admin-single-action-button \{[\s\S]*width: 2\.25rem !important/);
  assert.match(globalCssSource, /\.admin-card-actions\.admin-dual-actions \{[\s\S]*margin-left: -0\.625rem/);
});

test("wide admin tables turn into labelled row cards on mobile", () => {
  assert.match(pageSource, /admin-responsive-table admin-node-table/);
  assert.match(pingTaskSource, /admin-responsive-table/);
  assert.match(pingServerSource, /admin-responsive-table/);
  assert.match(offlineSource, /admin-responsive-table admin-selection-table/);
  assert.match(offlineSource, /common\.deselect_all[\s\S]*common\.select_all/);
  assert.doesNotMatch(offlineSource, /variant="soft"\s+color="gray"\s+className="md:hidden"/);
  assert.doesNotMatch(trafficReportSource, /variant="soft"\s+color="gray"\s+className="md:hidden"/);
  assert.match(offlineSource, /admin-single-text-action/);
  assert.match(loadSource, /admin-responsive-table admin-primary-first-table/);
  assert.match(pingLossSource, /admin-responsive-table admin-selection-table/);
  assert.match(pingLossSource, /TableHead className="text-center">\{t\("common\.action"\)\}/);
  assert.match(pingLossSource, /admin-card-actions admin-ping-loss-actions/);
  assert.match(pingLossSource, /type="button"\s+variant="soft"[\s\S]*onClick=\{toggleSelectAll\}[\s\S]*common\.deselect_all[\s\S]*common\.select_all/);
  assert.match(pingLossSource, /ConfigurationDialog targets=\{\[target\]\}[\s\S]*<Pencil size=\{16\} \/>/);
  assert.match(globalCssSource, /@media \(max-width: 767px\)/);
  assert.match(globalCssSource, /\.admin-responsive-table tbody tr/);
  assert.match(globalCssSource, /\.admin-responsive-table tbody tr \{[\s\S]*border-radius: calc\(var\(--radius\) - 2px\)/);
  assert.doesNotMatch(globalCssSource, /\.admin-selection-table tbody tr:first-child[\s\S]*border-top-left-radius: 0/);
  assert.match(globalCssSource, /content: attr\(data-label\)/);
});

test("desktop node table keeps readable name and network columns while resizing", () => {
  assert.match(
    pageSource,
    /admin-responsive-table admin-node-table min-w-\[1136px\] table-fixed/,
  );
  assert.match(pageSource, /TableCell className="w-\[44px\] px-2 !align-middle"/);
  assert.match(pageSource, /TableHead className="w-\[44px\]"/);
  assert.match(pageSource, /TableHead className="w-\[190px\]"/);
  assert.equal(pageSource.match(/TableHead className="w-\[190px\]"/g)?.length, 2);
  assert.match(pageSource, /TableHead className="w-\[72px\] text-center"/);
  assert.equal(pageSource.match(/TableHead className="w-\[72px\]/g)?.length, 3);
  assert.match(pageSource, /TableHead className="w-\[224px\]"/);
  assert.match(pageSource, /TableHead className="w-\[272px\]"/);
  assert.match(pageSource, /text-sm hover:bg-\[var\(--accent-a2\)\][^\n]*\[&>td\]:py-1\.5/);
  assert.match(pageSource, /text-sm leading-\[1\.125rem\]/);
  assert.match(pageSource, /data-label=\{t\("admin\.nodeTable\.name"\)\}[\s\S]{0,80}title=\{node\.name\}/);
  assert.match(pageSource, /\["IPv4", node\.ipv4\?\.trim\(\)\][\s\S]{0,80}\["IPv6", node\.ipv6\?\.trim\(\)\]/);
  assert.match(pageSource, /networkAddresses\.length > 0 \? networkAddresses\.map/);
  assert.doesNotMatch(pageSource, /IPv[46] \{node\.ipv[46] \|\| "--"\}/);
});

test("admin tables align selection controls and use available text width", () => {
  assert.match(pingLossSource, /TableHead className="w-12 px-3 text-center"/);
  assert.match(pingLossSource, /TableCell className="w-12 px-3" data-label=\{t\("common\.select"\)\}/);
  assert.doesNotMatch(loadSource, /slice\(0,\s*40\)/);
  assert.doesNotMatch(pingTaskSource, /slice\(0,\s*40\)/);
  assert.doesNotMatch(pingServerSource, /slice\(0,\s*40\)/);
  assert.match(loadSource, /min-w-0 flex-1 whitespace-normal break-words/);
  assert.match(pingTaskSource, /admin-responsive-table admin-sortable-table/);
  assert.match(pingTaskSource, /admin-card-actions admin-dual-actions flex items-center gap-3/);
  assert.match(loadSource, /admin-card-actions admin-dual-actions flex items-center gap-3/);
});

test("load notifications support default-on in add and edit flows", () => {
  assert.match(
    loadSource,
    /import \{ Checkbox \} from "@\/components\/ui\/checkbox"/,
  );
  assert.match(loadSource, /default_on: alert\.default_on \?\? false/);
  assert.match(loadSource, /default_on: newForm\.default_on/);
  assert.match(
    loadSource,
    /if \(!newForm\.default_on && newForm\.clients\.length === 0\)/,
  );
  assert.match(loadSource, /const \[defaultOn, setDefaultOn\] = React\.useState\(false\)/);
  assert.match(loadSource, /default_on: defaultOn/);
  assert.match(loadSource, /checked=\{form\.default_on\}/);
  assert.match(loadSource, /checked=\{defaultOn\}/);
  assert.match(loadSource, /ping\.default_on_description/);
});

test("batch selection uses one responsive toolbar instead of header checkboxes", () => {
  for (const source of [offlineSource, trafficReportSource, pingLossSource]) {
    assert.match(source, /AdminSelectionCount/);
    assert.match(source, /md:hidden/);
    assert.match(source, /summary=\{paginationSummary\}/);
    assert.doesNotMatch(source, /aria-label=\{t\("common\.select_all"/);
  }
  assert.match(selectionCountSource, /common\.selected_total/);
  assert.match(selectionCountSource, /split\(\/\(\\d\+\)\//);
});

test("mobile card action rows share the compact node-list layout", () => {
  assert.match(
    globalCssSource,
    /\.admin-responsive-table tbody td:last-child \{[\s\S]*grid-template-columns: minmax\(0, 1fr\)[\s\S]*padding-top: 0\.125rem !important/,
  );
  assert.match(
    globalCssSource,
    /\.admin-card-actions button \{[\s\S]*margin: 0 !important/,
  );
  assert.doesNotMatch(
    globalCssSource,
    /\.admin-node-table tbody td:last-child \{/,
  );
});

test("ping task search filters task and server views by task, target, or server", () => {
  assert.match(pingPageSource, /const \[search, setSearch\] = React\.useState\(""\)/);
  assert.match(pingPageSource, /task\.name[\s\S]*task\.target[\s\S]*serverNamesByUuid\.get\(uuid\)/);
  assert.match(pingPageSource, /<TaskView[\s\S]*pingTasks=\{filteredTasks\}[\s\S]*reorderEnabled=\{!search\.trim\(\)\}/);
  assert.match(pingPageSource, /<ServerView pingTasks=\{taskList\} search=\{search\} \/>/);
  assert.match(pingServerSource, /const filteredNodes = React\.useMemo/);
  assert.match(pingServerSource, /task\.name[\s\S]*task\.target/);
  assert.match(pingServerSource, /useAdminPagination\(filteredNodes\)/);
});

test("list toolbars and pagination follow the compact layout", () => {
  assert.match(pageSource, /size="2"[\s\S]*md:w-56/);
  assert.match(pageSource, /showSummary=\{false\}/);
  assert.match(loadSource, /<Tabs\.Content value="configuration"[\s\S]*<LoadListToolbar[\s\S]*showAdd[\s\S]*<LoadConfigurationTable/);
  assert.match(loadSource, /<Tabs\.Content value="current"[\s\S]*<LoadListToolbar[\s\S]*<CurrentLoadAlertsTable/);
  assert.match(loadSource, /className="min-w-0 flex-1 sm:max-w-64"/);
  assert.match(loadSource, /showSummary=\{false\}/);
  assert.match(offlineSource, /common\.select_all[\s\S]*batch_edit[\s\S]*order-last min-w-0 w-full basis-full sm:order-none sm:w-64 sm:basis-auto sm:flex-none/);
  assert.match(trafficReportSource, /common\.select_all[\s\S]*batch_edit[\s\S]*order-last min-w-0 w-full basis-full sm:order-none sm:w-64 sm:basis-auto sm:flex-none/);
  assert.equal(
    (trafficReportSource.match(/placeholder=\{t\("common\.search"\)\}/g) ?? []).length,
    1,
  );
  assert.match(pingLossSource, /common\.select_all[\s\S]*batch_edit[\s\S]*placeholder=\{t\("common\.search"\)\}[\s\S]*common\.add/);
  assert.match(pingLossSource, /md:ml-auto md:w-fit md:self-end/);
});
