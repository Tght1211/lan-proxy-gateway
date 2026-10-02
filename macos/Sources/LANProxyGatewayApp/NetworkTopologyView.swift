import SwiftUI

private struct NetworkNode: Identifiable {
    let id: String, title: String, icon: String, detail: String
    let column: Int
    let row: Double
    let destination: NetworkSheet
    var inactive = false
}
private struct NetworkFlight: Identifiable {
    let id = UUID()
    let connection: ConnectionInfo
    let path: [String]
    let start: Date
    var duration: Double {
        guard let ms = connection.responseMS else { return 1.8 }
        return min(3.0, max(0.8, 0.8 + Double(ms) / 2500))
    }
    var lifetime: Double { duration + 3 }
    var presentation: NetworkRequestPresentation { NetworkRequestPresentation(connection: connection) }
}

private struct NetworkTopologyIconAnchors: PreferenceKey {
    static var defaultValue: [String: Anchor<CGRect>] = [:]
    static func reduce(value: inout [String: Anchor<CGRect>], nextValue: () -> [String: Anchor<CGRect>]) {
        value.merge(nextValue(), uniquingKeysWith: { _, latest in latest })
    }
}

private struct NetworkTopologyIconPaths {
    let center: CGPoint
    let idle: Path
    let active: Path
    init(icon: String, bounds: CGRect) {
        center = CGPoint(x: bounds.midX, y: bounds.midY)
        let localBounds = CGRect(x: -bounds.width / 2, y: -bounds.height / 2, width: bounds.width, height: bounds.height)
        idle = StudioIcon(icon).outline(in: localBounds)
        active = StudioIcon(icon, active: true).outline(in: localBounds)
    }
}

private struct NetworkTopologyNodeLabel: View, Equatable {
    let id: String
    let title: String
    let detail: String
    let icon: String
    let inactive: Bool
    let probeText: String?
    let probeOK: Bool
    let width: CGFloat

    var body: some View {
        VStack(spacing: 9) {
            StudioIcon(icon)
                .frame(width: 30, height: 30)
                .hidden()
                .anchorPreference(key: NetworkTopologyIconAnchors.self, value: .bounds) { [id: $0] }
                .frame(height: 34)
            Text(title).font(.system(size: 12, weight: .medium))
                .foregroundStyle(inactive ? Theme.muted : Theme.text)
                .lineLimit(1).minimumScaleFactor(0.85)
            Text(detail).font(.system(size: 10)).foregroundStyle(Theme.muted).lineLimit(2)
            if let probeText {
                HStack(spacing: 4) {
                    Circle().fill(probeOK ? Theme.lime : Theme.coral).frame(width: 5, height: 5)
                    Text(probeText).font(.system(size: 10))
                }.foregroundStyle(probeOK ? Theme.lime : Theme.coral)
            }
        }.frame(width: width, height: 86).opacity(inactive ? 0.45 : 1)
    }
}

struct NetworkTopologyView: View {
    @EnvironmentObject private var model: AppModel
    @Environment(\.networkPageActive) private var isActive
    @Environment(\.accessibilityReduceMotion) private var reducedMotion
    @AppStorage("networkFlowPalette") private var palette: NetworkFlowPalette = .status
    @State private var feed = NetworkEventFeed()
    @State private var flights: [NetworkFlight] = []
    let onSelect: (NetworkSheet) -> Void
    private var network: HotspotStatus? { model.stats?.hotspot ?? model.hotspot }
    private var devices: [UsageAggregate] {
        let visible = model.visibleDevices
        return NetworkTopologyLayout.orderedDevices(visible, inactive: Set(visible.filter { model.isDeviceInactive($0) }.map(\.name)))
    }
    private var nodes: [NetworkNode] {
        var list: [NetworkNode] = devices.enumerated().map { index, d in
            NetworkNode(id: "device:" + d.name, title: model.effectiveDeviceLabel(for: d.name).nonEmpty ?? d.name,
                        icon: deviceIcon(label: model.effectiveDeviceLabel(for: d.name)), detail: shortBytes(d.total), column: 0, row: devices.count > 1 ? Double(index) * 3 / Double(devices.count - 1) : 1.5, destination: .device(d.name), inactive: model.isDeviceInactive(d))
        }
        let hotspotAddress = network?.ip.nonEmpty ?? "尚未发现"
        let localAddress = model.status?.gateway.localIP
        list.append(NetworkNode(id: "wifi", title: "Wi-Fi 热点", icon: "wifi", detail: network?.applied == true ? "已接管" : "未接管", column: 1, row: 0, destination: .access("wifi")))
        list.append(NetworkNode(id: "gateway", title: "静态网关", icon: "cable.connector", detail: "手动网关", column: 1, row: 1.5, destination: .access("gateway")))
        list.append(NetworkNode(id: "http", title: "HTTP / PAC", icon: "network", detail: model.status?.httpProxy?.enabled == true ? "已启用" : "未启用", column: 1, row: 3, destination: .access("http")))
        list.append(NetworkNode(id: "hotspot", title: "热点网关", icon: "wifi.router", detail: hotspotAddress, column: 2, row: 0.5, destination: .gateway(hotspotAddress)))
        list.append(NetworkNode(id: "lan", title: "局域网入口", icon: "server.rack", detail: localAddress?.nonEmpty ?? "尚未发现", column: 2, row: 2.5, destination: .gateway(localAddress ?? "—")))
        list.append(NetworkNode(id: "policy", title: "设备策略", icon: "slider.horizontal.3", detail: "设备优先", column: 3, row: 1.5, destination: .policy))
        list.append(NetworkNode(id: "rules", title: "流量规则", icon: "arrow.triangle.branch", detail: "首条命中", column: 4, row: 1.5, destination: .rules))
        list.append(NetworkNode(id: "learning", title: "自学习", icon: "sparkles", detail: "补充域名策略", column: 4, row: 3, destination: .learning))
        list.append(NetworkNode(id: "direct", title: "本机直连", icon: "globe", detail: "DIRECT", column: 5, row: 1.5, destination: .direct))
        list.append(NetworkNode(id: "reject", title: "拒绝", icon: "nosign", detail: "本机终止", column: 5, row: 3, destination: .reject))
        list.append(NetworkNode(id: "internet", title: "互联网", icon: "globe.americas", detail: "目标服务", column: 6, row: 1.5, destination: .internet))
        if model.status?.proxy?.isEmpty == false {
            list.append(NetworkNode(id: "proxy", title: "代理出口", icon: "cloud", detail: model.status?.proxy ?? "", column: 5, row: 0, destination: .proxy))
        }
        return list
    }
    private var links: [(String,String)] {
        var links = [("wifi","hotspot"),("gateway","lan"),("http","lan"),("hotspot","policy"),("lan","policy"),("policy","rules"),("rules","direct"),("rules","reject"),("direct","internet"),("learning","rules")]
        if nodes.contains(where: { $0.id == "proxy" }) { links += [("rules","proxy"),("proxy","internet")] }
        let connections = (model.stats?.relay.active ?? []) + (model.stats?.relay.recent ?? [])
        for d in devices {
            let entries = Set(connections.filter { $0.srcIP == d.name }.map { NetworkIngress.resolve($0, hotspot: network).rawValue })
            for entry in entries where entry != "unknown" { links.append(("device:" + d.name,entry)) }
        }
        return links
    }
    var body: some View {
        let topologyNodes = nodes
        let topologyLinks = links
        return Panel {
            VStack(alignment: .leading, spacing: 14) {
                HStack {
                    Text("连接航路").font(.headline)
                    Text("设备 → 规则 → 出口").font(.caption2).foregroundStyle(Theme.muted)
                    Spacer()
                    NetworkFlowPalettePicker()
                }
                GeometryReader { geo in
                    let positions = Dictionary(uniqueKeysWithValues: topologyNodes.map { ($0.id, position($0, width: geo.size.width, height: geo.size.height)) })
                    ZStack(alignment: .topLeading) {
                        Canvas { context, _ in
                            for (source, destination) in topologyLinks {
                                guard let sourcePosition = positions[source], let destinationPosition = positions[destination] else { continue }
                                let start = CGPoint(x: sourcePosition.x + (source == "learning" ? 0 : 22), y: sourcePosition.y + (source == "learning" ? -22 : 0))
                                let end = CGPoint(x: destinationPosition.x - (source == "learning" ? 0 : 22), y: destinationPosition.y + (source == "learning" ? 40 : 0))
                                context.stroke(curve(start, end), with: .color(Theme.border.opacity(0.75)), style: StrokeStyle(lineWidth: 1.3, dash: source == "learning" ? [3, 5] : []))
                            }
                        }.allowsHitTesting(false)
                        ForEach(Array(["设备","接入方式","Mac 网络入口","设备策略","流量规则","网络出口","目标服务"].enumerated()), id: \.offset) { column, title in
                            Text(title).font(.system(size: 11)).foregroundStyle(Theme.muted).frame(width: geo.size.width/7).position(x: geo.size.width*(Double(column)+0.5)/7,y:8)
                        }
                        ForEach(topologyNodes) { node in
                            let probe = node.id == "proxy" && model.stats?.egress == "proxy" ? model.stats?.health.history.last : nil
                            Button { onSelect(node.destination) } label: {
                                NetworkTopologyNodeLabel(id: node.id, title: node.title, detail: node.detail,
                                                         icon: node.icon, inactive: node.inactive,
                                                         probeText: probe.map { $0.ok ? formatMS($0.latencyMS) : "未响应" },
                                                         probeOK: probe?.ok ?? false, width: geo.size.width / 7 - 8)
                                    .equatable()
                            }.buttonStyle(.plain).help(node.title + " · " + node.detail)
                                .position(x: positions[node.id]!.x, y: positions[node.id]!.y + 25)
                        }
                    }
                    .overlayPreferenceValue(NetworkTopologyIconAnchors.self) { anchors in
                        GeometryReader { overlay in
                            let iconBounds = anchors.mapValues { overlay[$0] }
                            animatedLayer(nodes: topologyNodes, positions: positions, iconBounds: iconBounds,
                                          labelWidth: geo.size.width / 7 - 8)
                        }.allowsHitTesting(false)
                    }
                }.frame(height: NetworkTopologyLayout.minimumHeight(deviceCount: devices.count))
                HStack {
                    Text(devices.isEmpty ? "等待设备接入" : "真实连接采样 · 点击图标配置").font(.caption2).foregroundStyle(Theme.muted)
                    Spacer()
                    HStack(spacing: 10) {
                        legend(palette == .status ? "正常" : "同域名同色", color: palette.hues[0].color)
                        legend("> 500 ms", color: .yellow)
                        legend("错误", color: .red)
                    }.help(palette.detail)
                }
            }
        }.onChange(of: model.stats?.uptimeSec) { _ in sample() }.onAppear { sample() }
            .onChange(of: isActive) { active in
                if active { sample() }
                else { flights = []; feed.reset() }
            }
            .onDisappear { flights=[];feed.reset() }
    }
    private func animatedLayer(nodes: [NetworkNode], positions: [String: CGPoint],
                               iconBounds: [String: CGRect], labelWidth: CGFloat) -> some View {
        let iconPaths = Dictionary(uniqueKeysWithValues: nodes.compactMap { node in
            iconBounds[node.id].map { (node.id, NetworkTopologyIconPaths(icon: node.icon, bounds: $0)) }
        })
        return TimelineView(.animation(minimumInterval: reducedMotion ? 0.5 : 1.0 / 30,
                                       paused: !isActive || flights.isEmpty)) { timeline in
            Canvas { context, _ in
                let live = flights.filter { timeline.date.timeIntervalSince($0.start) >= 0 && timeline.date.timeIntervalSince($0.start) < $0.duration }
                for flight in live where !reducedMotion {
                    let flightColor = palette.hue(for: flight.presentation).color
                    let phase = timeline.date.timeIntervalSince(flight.start) / flight.duration
                    let segment = min(flight.path.count - 2, Int(phase * Double(flight.path.count - 1)))
                    guard segment >= 0, let source = positions[flight.path[segment]], let destination = positions[flight.path[segment + 1]] else { continue }
                    let local = phase * Double(flight.path.count - 1) - Double(segment)
                    for trailIndex in (0...4).reversed() {
                        let point = pointOnCurve(source, destination, t: max(0, local - Double(trailIndex) * 0.025))
                        let radius = trailIndex == 0 ? 3.0 : 2.0
                        context.fill(Path(ellipseIn: CGRect(x: point.x - radius, y: point.y - radius, width: radius * 2, height: radius * 2)),
                                     with: .color(flightColor.opacity(1 - Double(trailIndex) * 0.18)))
                    }
                }
                for node in nodes {
                    let burst = live.last { flight in
                        guard !reducedMotion else { return false }
                        let phase = timeline.date.timeIntervalSince(flight.start) / flight.duration
                        let lastIndex = flight.path.count - 1
                        let index = phase >= 0.85 ? lastIndex : min(lastIndex, max(0, Int(phase * Double(lastIndex))))
                        return flight.path[index] == node.id
                    }
                    let arrival = destinationFlight(for: node.id, at: timeline.date)
                    let tint = node.inactive ? Theme.muted : (burst ?? arrival).map { palette.hue(for: $0.presentation).color }
                        ?? (node.id == "reject" ? Theme.coral : node.id == "learning" || node.id == "proxy" ? Theme.cyan : Theme.text)
                    if let paths = iconPaths[node.id] {
                        var iconContext = context
                        iconContext.translateBy(x: paths.center.x, y: paths.center.y)
                        let scale = burst == nil || reducedMotion ? 1.0 : 1.06
                        iconContext.scaleBy(x: scale, y: scale)
                        iconContext.opacity = node.inactive ? 0.45 : 1
                        iconContext.stroke(burst == nil ? paths.idle : paths.active, with: .color(tint),
                                           style: StrokeStyle(lineWidth: 1.55, lineCap: .round, lineJoin: .round))
                    }
                    if let arrival, let position = positions[node.id] {
                        let elapsed = timeline.date.timeIntervalSince(arrival.start)
                        let begins = reducedMotion ? 0 : arrival.duration * 0.85
                        let motion = NetworkFloatingTextMotion.frame(at: (elapsed - begins) / (arrival.lifetime - begins), reducedMotion: reducedMotion)
                        var textContext = context
                        textContext.opacity = motion.opacity
                        let center = CGPoint(x: position.x, y: position.y - 38 - motion.lift)
                        drawFloatingText(id: arrival.id.uuidString + ":host", at: CGPoint(x: center.x, y: center.y - 8), context: &textContext)
                        drawFloatingText(id: arrival.id.uuidString + ":result", at: CGPoint(x: center.x, y: center.y + 8), context: &textContext)
                    }
                }
                for stage in ["policy", "rules"] where !reducedMotion {
                    let flight = live.last {
                        let phase = timeline.date.timeIntervalSince($0.start) / $0.duration
                        return stage == "policy" ? phase > 0.38 && phase < 0.58 : phase >= 0.58 && phase < 0.8
                    }
                    if let flight, let position = positions[stage] {
                        let phase = timeline.date.timeIntervalSince(flight.start) / flight.duration
                        let progress = stage == "policy" ? (phase - 0.38) / 0.2 : (phase - 0.58) / 0.22
                        let motion = NetworkFloatingTextMotion.frame(at: progress)
                        var textContext = context
                        textContext.opacity = 0.9 * motion.opacity
                        drawFloatingText(id: flight.id.uuidString + ":" + stage,
                                         at: CGPoint(x: position.x, y: position.y - 34 - motion.lift), context: &textContext)
                    }
                }
            } symbols: {
                ForEach(flights) { flight in
                    floatingTextSymbol(flight.connection.dstHost, color: Theme.muted, width: labelWidth)
                        .tag(flight.id.uuidString + ":host")
                    floatingTextSymbol(flight.presentation.label, color: palette.hue(for: flight.presentation).color, width: labelWidth)
                        .tag(flight.id.uuidString + ":result")
                    floatingTextSymbol(policyLabel(flight.connection), color: palette.hue(for: flight.presentation).color, width: labelWidth)
                        .tag(flight.id.uuidString + ":policy")
                    floatingTextSymbol(ruleLabel(flight.connection), color: palette.hue(for: flight.presentation).color, width: labelWidth)
                        .tag(flight.id.uuidString + ":rules")
                }
            }
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("最近采样连接")
        .accessibilityValue(flights.last.map { $0.connection.dstHost + "，" + $0.presentation.label } ?? "暂无新连接")
    }
    private func floatingTextSymbol(_ value: String, color: Color, width: CGFloat) -> some View {
        Text(value).font(.system(size: 10, weight: .medium)).foregroundStyle(color)
            .monospacedDigit().lineLimit(1).frame(width: width, height: 16)
    }
    private func drawFloatingText(id: String, at center: CGPoint, context: inout GraphicsContext) {
        guard let symbol = context.resolveSymbol(id: id) else { return }
        context.draw(symbol, at: center)
    }
    private func position(_ n: NetworkNode, width: Double, height: Double) -> CGPoint {
        CGPoint(x: width * (Double(n.column) + 0.5) / 7,
                y: NetworkTopologyLayout.verticalPosition(row: n.row, height: height))
    }
    private func legend(_ title: String, color: Color) -> some View {
        HStack(spacing: 4) { Circle().fill(color).frame(width: 4, height: 4); Text(title) }
            .font(.system(size: 9)).foregroundStyle(Theme.muted)
    }
    private func destinationFlight(for node: String, at date: Date) -> NetworkFlight? {
        guard node == "internet" || node == "reject" else { return nil }
        return flights.last {
            let elapsed = date.timeIntervalSince($0.start)
            return $0.path.last == node && elapsed >= (reducedMotion ? 0 : $0.duration * 0.85) && elapsed < $0.lifetime
        }
    }
    private func policyLabel(_ c: ConnectionInfo) -> String {
        switch model.deviceOverride(for: c.srcIP) {
        case "proxy": return "设备指定代理"
        case "direct": return "设备指定直连"
        case "reject": return "设备拒绝"
        default: return "遵循流量规则"
        }
    }
    private func ruleLabel(_ c:ConnectionInfo)->String {
        if c.outcome == .rejected { return "规则拒绝" }
        if c.fallback { return c.viaProxy ? "二次尝试代理":"降级直连" }
        if c.viaProxy { return "规则代理" }
        return "直连访问"
    }
    private func curve(_ a:CGPoint,_ b:CGPoint)->Path { var p=Path();p.move(to:a);p.addCurve(to:b,control1:CGPoint(x:(a.x+b.x)/2,y:a.y),control2:CGPoint(x:(a.x+b.x)/2,y:b.y));return p }
    private func pointOnCurve(_ a:CGPoint,_ b:CGPoint,t:Double)->CGPoint { let u=1-t,m=(a.x+b.x)/2;return CGPoint(x:u*u*u*a.x+3*u*u*t*m+3*u*t*t*m+t*t*t*b.x,y:u*u*u*a.y+3*u*u*t*a.y+3*u*t*t*b.y+t*t*t*b.y) }
    private func sample() {
        guard isActive else { return }
        guard let stats=model.stats else { flights=[];feed.reset();return }
        let fresh=feed.ingest(stats.relay.active+stats.relay.recent,uptime:stats.uptimeSec)
        flights.removeAll { Date().timeIntervalSince($0.start)>$0.lifetime }
        let known=Set(nodes.map(\.id))
        for (index,c) in fresh.suffix(8).enumerated() {
            let entry=NetworkIngress.resolve(c,hotspot:network)
            let rejected = c.outcome == .rejected
            let route=["device:"+c.srcIP,entry.rawValue,entry == .wifi ? "hotspot":"lan","policy","rules",rejected ? "reject":c.viaProxy ? "proxy":"direct"]+(rejected ? []:["internet"])
            guard route.allSatisfy(known.contains) else { continue }
            flights.append(NetworkFlight(connection:c,path:route,start:Date().addingTimeInterval(Double(index)*0.15)))
        }
        flights=Array(flights.suffix(16))
    }
}
