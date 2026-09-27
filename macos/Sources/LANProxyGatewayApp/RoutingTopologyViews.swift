import AppKit
import Charts
import SwiftUI
import UniformTypeIdentifiers

struct TopoAnchors: PreferenceKey {
    static var defaultValue: [String: Anchor<CGPoint>] = [:]
    static func reduce(value: inout [String: Anchor<CGPoint>], nextValue: () -> [String: Anchor<CGPoint>]) {
        value.merge(nextValue()) { $1 }
    }
}

extension View {
    func topoPort(_ id: String, _ edge: UnitPoint) -> some View {
        transformAnchorPreference(key: TopoAnchors.self, value: .unitPoint(edge)) { $0[id] = $1 }
    }
}

struct RouteDiagram: View {
    @EnvironmentObject private var model: AppModel
    let onEditRules: () -> Void
    let onEditProxy: () -> Void
    var historicalDevices: [UsageAggregate]? = nil
    var historicalIngress: [UsageAggregate]? = nil
    var historyRows: [DailyUsage] = []
    var onShowDevices: () -> Void = {}
    @State private var devicesExpanded = false
    @State private var showDevicePolicy = false
    @State private var showHotspotControls = false
    @State private var showHotspotGuide = false

    var body: some View {
        let rules = model.status?.routing ?? []
        let deviceRules = rules.filter { $0.type == "src-ip" }
        let domainRules = rules.filter { $0.type != "src-ip" }
        let egressProxy = model.status?.egress == "proxy"
        let allDevices = historicalDevices ?? model.stats?.relay.devices ?? []
        let devices = devicesExpanded ? Array(allDevices.prefix(8)) : Array(allDevices.prefix(3))
        let upstream = egressProxy ? (model.status?.proxy?.nonEmpty ?? "代理未配置") : "未配置 · 等价于直连"
        let activeConns = model.stats?.relay.active ?? []
        let observed = activeConns + (model.stats?.relay.recent ?? [])
        let httpIPs = Set(observed.filter { $0.isHTTPProxy }.map(\.srcIP) + historyRows.filter { $0.ingress == "http-proxy" }.map(\.device))
        let gatewayIPs = Set(observed.filter { !$0.isHTTPProxy }.map(\.srcIP) + historyRows.filter { $0.ingress == "gateway" }.map(\.device))
        let hotspot = model.stats?.hotspot
        let hotspotIPs = Set(gatewayIPs.filter { hotspot?.containsClient($0) == true })
        let hotspotActive = Set(activeConns.filter { !$0.isHTTPProxy && hotspotIPs.contains($0.srcIP) }.map { "dev.\($0.srcIP)" })
        let httpActive = Set(activeConns.filter { $0.isHTTPProxy }.map { "dev.\($0.srcIP)" })
        let gatewayActive = Set(activeConns.filter { !$0.isHTTPProxy && !hotspotIPs.contains($0.srcIP) }.map { "dev.\($0.srcIP)" })
        let activeIPs = Set(activeConns.map(\.srcIP))
        let flowProxy = activeConns.contains { $0.viaProxy }
        let flowDirect = activeConns.contains { !$0.viaProxy }
        let recentRejectCutoff = Date().addingTimeInterval(-120)
        let flowReject = (model.stats?.relay.recent ?? []).contains { $0.rejected && $0.startedAt > recentRejectCutoff }
        let hasDeviceOverrides = !deviceRules.isEmpty
        let protectedCount = model.stats?.deviceAdaptive?.devices.filter { $0.mode == "direct" }.count ?? 0

        VStack(spacing: 0) {
            TopoStage(portID: "router", icon: "wifi.router", tint: Theme.lime,
                      title: "主路由 · 互联网", detail: model.status?.gateway.router.nonEmpty ?? "光猫/路由器")
                .help("代理和直连的流量最终都经主路由访问互联网；设备可通过网关或手动 HTTP 代理接入")
            Spacer(minLength: 18)
            HStack(alignment: .top, spacing: 14) {
                TopoOutcome(icon: "cloud.fill", title: "上游代理", detail: upstream,
                            count: ruleCount(rules, "proxy"), color: Theme.cyan,
                            emphasized: egressProxy,
                            badge: egressProxy ? "默认" : "回退直连",
                            badgeColor: egressProxy ? Theme.cyan : Theme.yellow,
                            breathing: true,
                            help: egressProxy ? "点击配置网络出口" : "未配置上游代理，proxy 规则会回退为直连；点击配置",
                            action: onEditProxy)
                    .topoPort("out.proxy", .bottom)
                    .topoPort("out.proxy.t", .top)
                TopoOutcome(icon: "network", title: "本机直连", detail: "DIRECT",
                            count: ruleCount(rules, "direct"), color: Theme.lime,
                            emphasized: !egressProxy,
                            badge: egressProxy ? nil : "默认",
                            help: "点击查看命中直连的规则",
                            ruleList: rules.filter { $0.action == "direct" },
                            onEditRules: onEditRules, action: {})
                    .topoPort("out.direct", .bottom)
                    .topoPort("out.direct.t", .top)
                TopoOutcome(icon: "nosign", title: "拒绝", detail: "REJECT",
                            count: ruleCount(rules, "reject"), color: Theme.coral,
                            emphasized: false,
                            help: "点击查看被拒绝的规则",
                            ruleList: rules.filter { $0.action == "reject" },
                            onEditRules: onEditRules, action: {})
                    .topoPort("out.reject", .bottom)
            }
            Spacer(minLength: 18)
            Button(action: onEditRules) {
                TopoStage(
                    portID: "rules", icon: "arrow.triangle.branch", tint: Theme.yellow,
                    title: "② 共用分流规则",
                    detail: domainRules.isEmpty ? "点击配置分流规则" : "\(domainRules.count) 条 · 首条命中",
                    clickable: true
                )
            }
            .buttonStyle(.plain)
            .help("按域名、域名后缀、IP-CIDR 判断走代理、直连还是拒绝")
            Spacer(minLength: 18)
            Button { showDevicePolicy = true } label: {
                TopoStage(
                    portID: "devpolicy", icon: "person.crop.circle.badge.checkmark", tint: Theme.coral,
                    title: "① 设备策略",
                    detail: devicePolicySummary(deviceRules: deviceRules, protectedCount: protectedCount),
                    clickable: true
                )
            }
            .buttonStyle(.plain)
            .help("设备级前置策略：优先于域名规则，可为每台设备指定代理/直连/拒绝")
            .popover(isPresented: $showDevicePolicy, arrowEdge: .trailing) {
                DevicePolicyPopover()
                    .environmentObject(model)
            }
            Spacer(minLength: 18)
            TopoStage(portID: "gw", icon: "server.rack", tint: Theme.cyan,
                      title: "旁路由", detail: hotspot?.ip.nonEmpty ?? model.status?.gateway.localIP.nonEmpty ?? "--")
            Spacer(minLength: 18)
            HStack(spacing: 16) {
                if model.status?.accessMode == "hotspot" {
                    Button { showHotspotControls = true } label: {
                        TopoStage(portID: "ingress.hotspot", icon: "wifi", tint: Theme.cyan,
                                  title: "代理 Wi-Fi", detail: "\(hotspot?.applied == true ? "接管已开启" : "等待接管") · \(hotspotActive.count) 台活跃",
                                  clickable: true)
                    }
                    .buttonStyle(.plain)
                    .help("点击管理 Wi-Fi：名称与密码、接管开关、分流规则")
                    .popover(isPresented: $showHotspotControls, arrowEdge: .bottom) {
                        HotspotQuickControls(
                            onRules: { showHotspotControls = false; onEditRules() },
                            onGuide: { showHotspotControls = false; showHotspotGuide = true }
                        ).environmentObject(model)
                    }
                }
                TopoStage(portID: "ingress.gateway", icon: "network", tint: Theme.lime,
                          title: "手动网关", detail: "\(gatewayActive.count) 台活跃")
                TopoStage(portID: "ingress.http", icon: "globe", tint: Theme.cyan,
                          title: "HTTP 代理", detail: ingressSummary("http-proxy", active: httpActive.count))
                    .help("显示手动与 PAC HTTP 代理的连接及流量；服务端无法区分客户端的配置方式。穿透连接显示隧道客户端的来源 IP。")
            }
            Spacer(minLength: 18)
            HStack(spacing: 12) {
                if devices.isEmpty {
                    TopoDeviceChip(icon: "desktopcomputer", title: "等待设备接入", subtitle: "连接代理 Wi-Fi 或选择其他接入方式", active: false)
                        .topoPort("dev.empty", .top)
                } else {
                    ForEach(devices) { device in
                        let label = model.effectiveDeviceLabel(for: device.name).nonEmpty
                        TopoDeviceChip(
                            icon: deviceIcon(label: label),
                            title: label ?? device.name,
                            subtitle: "\(shortBytes(device.total)) · \(activeIPs.contains(device.name) ? "当前活跃" : "曾接入")",
                            active: activeIPs.contains(device.name)
                        )
                        .topoPort("dev.\(device.name)", .top)
                    }
                    if allDevices.count > 3 {
                        Button {
                            onShowDevices()
                        } label: {
                            VStack(spacing: 2) {
                                Image(systemName: devicesExpanded ? "chevron.left" : "chevron.right")
                                    .font(.system(size: 10, weight: .bold))
                                Text(devicesExpanded ? "收起" : "+\(allDevices.count - 3)")
                                    .font(.system(size: 9, weight: .bold, design: .monospaced))
                            }
                            .foregroundStyle(Theme.muted)
                            .frame(width: 34, height: 40)
                            .background(Theme.panelRaised)
                            .clipShape(RoundedRectangle(cornerRadius: Theme.radius))
                        }
                        .buttonStyle(.plain)
                        .help(devicesExpanded ? "收起设备" : "展开全部设备")
                    }
                }
            }
        }
        .sheet(isPresented: $showHotspotGuide) { DeviceOnboardingSheet().environmentObject(model) }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .backgroundPreferenceValue(TopoAnchors.self) { anchors in
            GeometryReader { geo in
                TopoLinkLayer(
                    points: anchors.mapValues { geo[$0] },
                    devicePorts: devices.isEmpty ? ["dev.empty"] : devices.map { "dev.\($0.name)" },
                    httpDevicePorts: Set(devices.filter { httpIPs.contains($0.name) }.map { "dev.\($0.name)" }),
                    gatewayDevicePorts: Set(devices.filter { (gatewayIPs.contains($0.name) || !httpIPs.contains($0.name)) && !hotspotIPs.contains($0.name) }.map { "dev.\($0.name)" }),
                    hotspotDevicePorts: Set(devices.filter { hotspotIPs.contains($0.name) }.map { "dev.\($0.name)" }),
                    hotspotActivePorts: hotspotActive,
                    httpActivePorts: httpActive,
                    gatewayActivePorts: gatewayActive,
                    active: model.isRunning,
                    flowProxy: flowProxy,
                    flowDirect: flowDirect,
                    flowReject: flowReject,
                    egressProxy: egressProxy,
                    hasDeviceOverrides: hasDeviceOverrides
                )
            }
        }
    }

    private func ingressSummary(_ name: String, active: Int) -> String {
        let usage = (historicalIngress ?? model.stats?.relay.ingress ?? []).first { $0.name == name }
        let total = (usage?.up ?? 0) + (usage?.down ?? 0)
        let traffic = "\(active) 台活跃 · \(shortBytes(total))"
        if name == "http-proxy" {
            guard model.status?.httpProxy?.enabled == true else { return "未开启 · \(shortBytes(total))" }
            let listening = model.stats?.components?.contains { $0.name == "http-proxy" && $0.running } == true
            if !model.isRunning || !listening { return "未监听 · \(shortBytes(total))" }
        }
        return traffic
    }

    private func ruleCount(_ rules: [RoutingRule], _ action: String) -> Int {
        rules.filter { $0.action == action }.count
    }

    private func devicePolicySummary(deviceRules: [RoutingRule], protectedCount: Int) -> String {
        if deviceRules.isEmpty && protectedCount == 0 { return "全部默认 · 流向域名规则" }
        var parts: [String] = []
        let proxyCount = deviceRules.filter { $0.action == "proxy" }.count
        let directCount = deviceRules.filter { $0.action == "direct" }.count
        let rejectCount = deviceRules.filter { $0.action == "reject" }.count
        if proxyCount > 0 { parts.append("\(proxyCount) 代理") }
        if directCount > 0 { parts.append("\(directCount) 直连") }
        if rejectCount > 0 { parts.append("\(rejectCount) 拒绝") }
        if protectedCount > 0 { parts.append("\(protectedCount) 保护") }
        return parts.joined(separator: " · ")
    }
}

// MARK: - Device Policy Popover (Topo ① click target)
