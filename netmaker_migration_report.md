
# Evaluating Netmaker for BGP4mesh

**Date:** 2025-11-06  
**Author:** @ai.altermundi (with ChatGPT)  
**Scope:** Assess replacing **tinc + etcd (+ parts of the custom Go daemon)** with **Netmaker (WireGuard-based L3 overlay)** in the current stack.

---

## TL;DR

- **Viable swap:** If we can move from **Layer 2 (tinc switch mode)** to **Layer 3 (WireGuard)** and accept a **central control plane** (Netmaker server + DB + MQTT), Netmaker can replace **tinc** and **etcd** and simplify key/peer/config management.
- **Biggest change:** Loss of native L2 broadcast/multicast across the mesh. Many workflows survive with **DNS-first** and static IPs; true L2 needs **workarounds** (VXLAN over WG or mDNS reflectors).
- **Main wins:** Performance (WireGuard), simpler orchestration, route-based control, and ready-made gateways/ACLs.
- **Main risks:** Control-plane dependency/HA, L2 gaps, and learning curve for Netmaker components.

---

## Our current stack (baseline)

- **BIRD 3.x** — MP-BGP, RPKI validation and routing policy
- **tinc 1.0 (switch/L2)** — L2 mesh, RSA-2048/AES-256; broadcasts/multicast work end-to-end
- **etcd 3.5+** — Distributed key-value for config/state
- **Go daemon** — mDNS discovery, key distribution, config sync
- **Ansible** — Orchestration
- **Prometheus + Grafana** — Metrics/monitoring
- **Docker** — Service containerization

**Key property today:** L2 semantics across the mesh (tinc switch mode) + decentralized-ish peer configs; discovery via mDNS is straightforward because broadcasts traverse the L2 overlay.

---

## What Netmaker is (in one minute)

- **Data plane:** **WireGuard** tunnels (kernel space, fast, modern crypto).
- **Control plane:** **Netmaker server** manages networks and pushes configs to **netclient** agents via **MQTT**.
- **State store:** Config/state in a DB (commonly **Postgres**; SQLite/rqlite also seen). For HA, Postgres + replicated server is the common path.
- **Networking model:** **L3 overlay** (no native broadcast/multicast). Adds features like **egress/ingress gateways**, **relays**, **ACLs**, and optional **DNS (CoreDNS)** for in-mesh names.
- **Packaging:** Docker/K8s friendly; CLI + UI. Community vs Pro split (exporters/dashboards and some features in Pro).

---

## Mapping: what Netmaker replaces

| Current Component | Role today | With Netmaker |
|---|---|---|
| **tinc (L2)** | L2 mesh VPN carrying all traffic, broadcasts included | **Replaced by Netmaker (L3 WG overlay)**. **No native L2.** Routes/IPs instead of broadcasts. |
| **etcd** | Shared config/state KV | **Replaced by Netmaker’s DB** (server-managed state). |
| **Go daemon** | mDNS, key distribution, config sync | **Mostly replaced** (server handles keys/peers/config). mDNS needs a new approach (see below). |
| **BIRD** | BGP/MP-BGP/RPKI | **Stays**. Peers run over the WireGuard interfaces Netmaker creates (e.g. `wg0`). |
| **Ansible** | Infra orchestration | **Optional**; still useful for bootstrapping netclient/server and host ops. |
| **Prometheus/Grafana** | Metrics | **Unchanged**. Optionally add Netmaker’s metrics/exporter if desired. |

---

## Pros and cons of swapping to Netmaker

### ✅ Pros

1. **Performance & efficiency**  
   - WireGuard in kernel space usually outperforms userland VPNs like tinc. Lower latency; high throughput.
2. **Simplified peer/key management**  
   - Centralized controller issues/join-keys, rotates keys, defines networks/ACLs—less custom glue.
3. **Cleaner L3 routing model**  
   - Plays naturally with BGP. Easy to separate control/data traffic, define subnets, and use gateways/relays.
4. **Operational tooling**  
   - Admin UI/CLI, Docker/K8s deploys, DNS integration, access control lists, automatic peer updates.
5. **Security posture**  
   - Modern cryptography (Curve25519/ChaCha20-Poly1305 via WireGuard). Reduced attack surface vs bespoke PKI automation.

### ⚠️ Cons / Risks

1. **No native Layer 2**  
   - Broadcast/multicast don’t traverse by default. mDNS-based flows break unless replaced or tunneled (VXLAN/reflector).
2. **Central control plane dependency**  
   - Requires **Netmaker server + DB + MQTT**. For **HA**, we likely want K8s or at least multiple replicas + Postgres-HA.
3. **Community vs Pro features**  
   - Some monitoring/analytics and conveniences live in Pro. Confirm whether Community meets our needs.
4. **Migration/learning curve**  
   - Addressing plans, DNS-first thinking, and ACLs need initial design. Team needs to learn server + netclient lifecycle.
5. **Operational blast radius**  
   - Controller outage doesn’t drop existing tunnels, but new joins/updates pause. DB availability matters for changes.

---

## L2 semantics: do we need them? (decision hinge)

- If **we don’t require L2** (broadcast/multicast), Netmaker is a strong drop-in for the transport.
- If **we do require L2** in parts of the mesh, pick one:
  1. **Reduce L2 scope** — keep L2 only where needed (local segments) and use **L3 routing** across sites/regions.
  2. **mDNS reflectors** — run an mDNS/Bonjour reflector (e.g., Avahi reflector) across select nodes.
  3. **VXLAN over WireGuard** — encapsulate L2 segments over the WG L3 mesh for specific VLANs only. More ops cost.

**Recommendation:** Prefer **DNS-first** services and explicit IPs; add **reflector or VXLAN** for the handful of L2-specific use cases if truly needed.

---

## Security and compliance notes

- **Crypto:** WireGuard (Curve25519, ChaCha20-Poly1305) vs tinc’s RSA/AES—modern defaults, shorter handshake, good posture.
- **Identity & access:** Join-keys/tokens gate membership; server controls network ACLs; rotate keys regularly.
- **Secrets:** Protect server DB credentials, MQTT creds, admin UI. Treat join tokens like privileged secrets.
- **RBAC & audit:** Decide who can create networks, gateways, ACLs; capture changes (GitOps for NM config where possible).
- **RPKI:** Unchanged—still in BIRD. Ensure WG interfaces are restricted to BGP peers we expect (ACLs).

---

## High-level architecture with Netmaker

- **Netmaker server** (behind TLS) + **DB** (prefer **Postgres** for HA) + **MQTT broker** (e.g., Mosquitto).
- **netclient** on every node creates one or more **wgX** interfaces per “network”.
- **BIRD** sessions ride over these wg interfaces (static neighbor IPs, templated via Ansible if desired).
- Optional: **CoreDNS** to resolve in-mesh names; **egress/ingress gateways** for reaching/advertising external subnets.

---

## Proposed pilot (1 week)

1. **Bootstrap** Netmaker server on a VM (or on K8s if we want HA from day 0). Use **Postgres**.
2. **Create one L3 network** (e.g., `nm-mesh`) and enroll 3–5 representative nodes with **netclient**.
3. **Assign addressing**: /24 or /25 inside 100.64.0.0/10 (CGNAT) or another RFC1918 chunk reserved for the overlay.
4. **BIRD peering test**: Bring up BGP neighbors over `wg0`; confirm route exchange and convergence.
5. **Discovery migration**: Replace mDNS-dependent lookups with **DNS names** issued via Netmaker/CoreDNS. If something breaks, test **Avahi reflector** or **VXLAN** between two sites.
6. **Measure**: Latency, throughput, and route changes vs tinc. Document results and any surprises.
7. **Decide**: If all green, proceed to staged migration.

---

## Staged migration plan (production)

**Stage 0 – Design**
- Address plan per network; ACL model; gateway/relay placements; DB/backup plan; SLO for controller.
- Pick HA strategy: K8s (preferred) or multi-VM with keepalived/proxy + Postgres-HA.

**Stage 1 – Dual-run**
- Run **tinc (L2)** and **Netmaker (L3)** in parallel on a subset of nodes. BIRD peers over WG on those nodes.
- Cut over DNS names for services that don’t require L2 semantics.

**Stage 2 – Expand L3**
- Migrate remaining BIRD peering to WG overlay.
- For any L2-only services, decide per-case: **reflector**, **VXLAN**, or keep a **small tinc island**.

**Stage 3 – Decommission**
- Once traffic is fully on WG, decommission **tinc** where no longer needed.
- Migrate config/state out of **etcd** (retire it) and remove custom Go sync where NM covers the need.

**Stage 4 – Harden & observe**
- Add backups for Postgres, monitor server/DB/MQTT health, enable alerting.
- Document join/offboarding procedures; rotate keys; export metrics to Prometheus/Grafana.

---

## Operational runbook (sketch)

- **Backups**: Daily logical backups of Postgres + weekly base + WAL shipping if desired.
- **Upgrades**: Test NM server upgrades in staging; netclients auto-refresh configs.
- **Onboarding**: Issue join-key, run `netclient join` with the proper network; verify wg interface and routes.
- **Offboarding**: Remove node from NM; revoke keys; routes withdrawn automatically.
- **Monitoring**: Uptime for server/MQTT/DB; tunnel health; BGP sessions on BIRD; exporter if using Pro.

---

## BIRD over WireGuard (example snippet)

> Adjust to your addressing. Assuming WG overlay 100.70.0.0/16 and a neighbor at 100.70.10.2 on `wg0`.

```conf
# bird.conf (BIRD 3.x style – simplify as needed)

router id 10.0.0.1;

protocol device {
}

protocol kernel kernel4 {
  ipv4 { export all; import none; };
}

protocol static static4 {
  ipv4;
  route 100.70.10.1/32 blackhole;  # Ensure local /32 reachable if needed
}

template bgp bgp_wg {
  local as 65001;
  ipv4 {
    import all;
    export where source = RTS_STATIC || source = RTS_BGP;
  };
  multihop 2;          # often 1 hop is fine; adjust for your topology
  graceful restart on;
  connect delay 2;
  hold time 30;
}

protocol bgp peer_site_a from bgp_wg {
  neighbor 100.70.10.2 as 65002;   # neighbor over wg0
  interface "wg0";
}
```

---

## Decision checklist

- [ ] We can operate in **L3** for all/most services.
- [ ] For necessary L2 cases, we chose **reflector** or **VXLAN** (documented).
- [ ] We accept a **controller** and have an **HA** plan (prefer K8s + Postgres).
- [ ] Address plan and ACLs are defined; DNS strategy agreed.
- [ ] Monitoring + backup runbooks ready.
- [ ] Rollback plan defined (see below).

---

## Rollback plan

- Keep **tinc** configs intact during dual-run.
- If Netmaker controller/DB fails during rollout, **BIRD can continue** over existing WG tunnels, but **new changes freeze**; if needed, flip BIRD peers back to interfaces on the tinc overlay and disable netclient until controller is healthy.
- If performance/regression observed, revert peers to tinc, stop netclient, and remove NM routes.

---

## References (starting points)

- Netmaker: https://github.com/gravitl/netmaker  
- Netmaker docs: https://docs.netmaker.org (install, HA, DNS, gateways, ACLs)  
- WireGuard: https://www.wireguard.com  
- tinc: https://www.tinc-vpn.org/  
- BIRD: https://bird.network.cz/

---

## Bottom line

If we **don’t need L2 end-to-end**, Netmaker is a compelling simplification: faster data plane, cleaner control, and fewer custom parts. If **L2 is essential**, keep tinc where necessary (or tunnel L2 selectively) and run a **hybrid** design.
