# Public address inventory (path monitoring groundwork)

The Agent continues to report its existing `ipv4` and `ipv6` strings unchanged. These are the **primary/reported egress addresses** selected by the existing Agent configuration and detection logic; they are not necessarily assigned to a local interface when NAT is involved.

The additive `ip_addresses` basic-info field is an array of objects:

```json
[
  {"address":"8.8.8.8","family":"ipv4","interface":"eth0","source":"interface","primary":true},
  {"address":"9.9.9.9","family":"ipv4","interface":"eth1","source":"interface"},
  {"address":"2001:4860::1","family":"ipv6","source":"reported","primary":true}
]
```

`address` is a canonical IP literal, `family` is `ipv4` or `ipv6`, `interface` identifies a local assignment when known, `source` is currently `interface` or `reported`, and `primary` marks an identity equal to the corresponding legacy scalar. The list includes primary public addresses **and** distinct additional addresses. It contains only addresses that pass the Agent's public-address filter, not all private, link-local, documentation, shared/CGNAT, or benchmark addresses. The Agent scans all *up* interfaces independently of traffic-monitoring NIC filters, including public addresses assigned to an up loopback interface. It cannot discover secondary external NAT mappings that are not assigned locally or reported through the primary egress detector. The current Agent/backend bound a report to 1,024 addresses and the backend accepts up to 512 KiB of encoded JSON.

The backend persists the list as JSON text in `clients.ip_addresses` (schema version 2). Existing `ipv4`/`ipv6` columns and their administrator API fields remain intact; older Agents can omit the list. An older Agent reporting updated scalar addresses clears a previously stored list rather than leaving stale extra addresses. Administrator `getNodes` returns the list; guest/public node responses remove it, regardless of whether guest primary addresses are visible. The administrator table and detail dialog show primary scalars by default and extra addresses only inside a collapsed, scrollable disclosure.

This is an inventory of **candidate identities**, not a proof of reachability or a route selection. Do not infer path latency, per-destination source address, transit-vs-landing roles, traffic attribution, route priority, or egress from this list. Future path monitoring should identify source and destination nodes/identities explicitly, record observation time, protocol/family and actual route/source selection, and attach latency and traffic measurements to those observations rather than overloading this inventory. Later optional per-identity metadata can be added to the structured entries without changing the legacy scalar contract.

Deploying the backend/frontend alone does not populate secondary addresses on existing nodes: their Agents must also be updated, after which the next basic-info report fills the inventory. This change does not include a production rollout, new Release, or path-monitoring probes.
