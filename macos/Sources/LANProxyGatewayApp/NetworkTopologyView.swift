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
    var color: Color { [Color.purple, .teal, .orange, .pink, .blue, .indigo][NetworkDomainColor.index(connection.dstHost)] }
}

struct NetworkTopologyView: View {
    @EnvironmentObject private var model: AppModel
    @Environment(\.accessibilityReduceMotion) private var reducedMotion
    @AppStorage("networkMotion") private var motion = "subtle"
    @State private var paused = false
    @State private var feed = NetworkEventFeed()
    @State private var flights: [NetworkFlight] = []
    let onSelect: (NetworkSheet) -> Void
    private var network: HotspotStatus? { model.stats?.hotspot ?? model.hotspot }
    private var devices: [UsageAggregate] { Array(model.visibleDevices.prefix(4)) }
    private var nodes: [NetworkNode] {
        var list = devices.enumerated().map { index, d in
            NetworkNode(id: "device:" + d.name, title: model.effectiveDeviceLabel(for: d.name).nonEmpty ?? d.name,
                        icon: deviceIcon(label: model.effectiveDeviceLabel(for: d.name)), detail: shortBytes(d.total), column: 0, row: devices.count > 1 ? Double(index) * 3 / Double(devices.count - 1) : 1.5, destination: .device(d.name), inactive: model.isDeviceInactive(d))
        }
        list += [
            .init(id: "wifi", title: "Wi-Fi 热点", icon: "wifi", detail: network?.applied == true ? "已接管" : "未接管", column: 1, row: 0, destination: .access("wifi")),
            .init(id: "gateway", title: "静态网关", icon: "cable.connector", detail: "手动网关", column: 1, row: 1.5, destination: .access("gateway")),
            .init(id: "http", title: "HTTP / PAC", icon: "network", detail: model.status?.httpProxy?.enabled == true ? "已启用" : "未启用", column: 1, row: 3, destination: .access("http")),
            .init(id: "hotspot", title: "热点网关", icon: "wifi.router", detail: network?.ip.nonEmpty ?? "尚未发现", column: 2, row: 0.5, destination: .gateway(network?.ip.nonEmpty ?? "尚未发现")),
            .init(id: "lan", title: "局域网入口", icon: "server.rack", detail: model.status?.gateway.localIP.nonEmpty ?? "尚未发现", column: 2, row: 2.5, destination: .gateway(model.status?.gateway.localIP ?? "—")),
            .init(id: "policy", title: "设备策略", icon: "slider.horizontal.3", detail: "设备优先", column: 3, row: 1.5, destination: .policy),
            .init(id: "rules", title: "流量规则", icon: "arrow.triangle.branch", detail: "首条命中", column: 4, row: 1.5, destination: .rules),
            .init(id: "learning", title: "自学习", icon: "sparkles", detail: "补充域名策略", column: 4, row: 3, destination: .learning),
            .init(id: "direct", title: "本机直连", icon: "globe", detail: "DIRECT", column: 5, row: 1.5, destination: .direct),
            .init(id: "reject", title: "拒绝", icon: "nosign", detail: "本机终止", column: 5, row: 3, destination: .reject),
            .init(id: "internet", title: "互联网", icon: "globe.americas", detail: "目标服务", column: 6, row: 1.5, destination: .internet)
        ]
        if model.status?.proxy?.isEmpty == false {
            list.append(.init(id: "proxy", title: "代理出口", icon: "cloud", detail: model.status?.proxy ?? "", column: 5, row: 0, destination: .proxy))
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
        Panel {
            VStack(alignment: .leading, spacing: 18) {
                HStack { Text("连接航路").font(.headline); Spacer(); Text("设备 → 规则 → 出口").font(.caption).foregroundStyle(Theme.muted); Button(paused ? "播放光流" : "暂停光流") { paused.toggle(); flights = [] }.buttonStyle(StudioButtonStyle()).disabled(reducedMotion || motion == "off") }
                GeometryReader { geo in
                    TimelineView(.animation(minimumInterval: 1.0 / 30, paused: paused || reducedMotion || motion == "off" || flights.isEmpty)) { timeline in
                        let positions = Dictionary(uniqueKeysWithValues: nodes.map { ($0.id, position($0, width: geo.size.width)) })
                        let live = flights.filter { timeline.date.timeIntervalSince($0.start) >= 0 && timeline.date.timeIntervalSince($0.start) < $0.duration }
                        ZStack(alignment: .topLeading) {
                            Canvas { context, _ in
                                for (a,b) in links {
                                    guard let p = positions[a], let q = positions[b] else { continue }
                                    let start = CGPoint(x:p.x + (a == "learning" ? 0:22),y:p.y + (a == "learning" ? -22:0))
                                    let end = CGPoint(x:q.x - (a == "learning" ? 0:22),y:q.y + (a == "learning" ? 40:0))
                                    context.stroke(curve(start,end), with: .color(Theme.border.opacity(0.75)), style: StrokeStyle(lineWidth: 1.3, dash: a == "learning" ? [3,5] : []))
                                }
                                for flight in live {
                                    let phase = timeline.date.timeIntervalSince(flight.start) / flight.duration
                                    let segment = min(flight.path.count-2, Int(phase * Double(flight.path.count-1)))
                                    guard segment >= 0, let a = positions[flight.path[segment]], let b = positions[flight.path[segment+1]] else { continue }
                                    let local = phase * Double(flight.path.count-1) - Double(segment)
                                    for i in (0...4).reversed() {
                                        let point = pointOnCurve(a,b,t:max(0,local - Double(i)*0.025))
                                        let radius = i == 0 ? 3.0 : 2.0
                                        context.fill(Path(ellipseIn: CGRect(x:point.x-radius,y:point.y-radius,width:radius*2,height:radius*2)), with:.color(flight.color.opacity(1-Double(i)*0.18)))
                                    }
                                }
                            }.allowsHitTesting(false)
                            ForEach(Array(["设备","接入方式","Mac 网络入口","设备策略","流量规则","网络出口","目标服务"].enumerated()), id: \.offset) { column, title in
                                Text(title).font(.system(size: 11)).foregroundStyle(Theme.muted).frame(width: geo.size.width/7).position(x: geo.size.width*(Double(column)+0.5)/7,y:8)
                            }
                            ForEach(nodes) { node in
                                let burst = live.last { flight in
                                    let phase = timeline.date.timeIntervalSince(flight.start) / flight.duration
                                    let lastIndex = flight.path.count - 1
                                    let index = min(lastIndex, max(0, Int(phase * Double(lastIndex))))
                                    return flight.path[index] == node.id
                                }
                                Button { onSelect(node.destination) } label: {
                                    VStack(spacing: 9) {
                                        StudioIcon(node.icon, active: burst != nil)
                                            .frame(width: 30, height: 30)
                                            .foregroundStyle(node.inactive ? Theme.muted : burst?.color ?? (node.id == "reject" ? Theme.coral : node.id == "learning" || node.id == "proxy" ? Theme.cyan : Theme.text))
                                            .scaleEffect(burst == nil ? 1 : 1.06)
                                            .animation(.easeOut(duration:0.18),value:burst?.id)
                                            .frame(height: 34)
                                        Text(node.title).font(.system(size:12,weight:.medium)).foregroundStyle(node.inactive ? Theme.muted : Theme.text).lineLimit(1)
                                        Text(node.detail).font(.system(size:10)).foregroundStyle(Theme.muted).lineLimit(2)
                                        if node.id == "proxy", model.stats?.egress == "proxy", let latest = model.stats?.health.history.last {
                                            HStack(spacing: 4) { Circle().fill(latest.ok ? Theme.lime : Theme.coral).frame(width: 5,height: 5); Text(latest.ok ? formatMS(latest.latencyMS) : "未响应").font(.system(size: 10)) }.foregroundStyle(latest.ok ? Theme.lime : Theme.coral)
                                        }
                                    }.frame(width:geo.size.width/7-8,height:86).opacity(node.inactive ? 0.45 : 1)
                                }.buttonStyle(.plain).help(node.title + " · " + node.detail)
                                    .position(x:positions[node.id]!.x,y:positions[node.id]!.y+25)
                            }
                            ForEach(live) { flight in
                                let phase = timeline.date.timeIntervalSince(flight.start) / flight.duration
                                let stage = phase < 0.58 ? "policy" : phase < 0.8 ? "rules" : flight.connection.rejected ? "reject" : "internet"
                                if phase > 0.38, let p = positions[stage] {
                                    Text(stage == "policy" ? policyLabel(flight.connection) : stage == "rules" ? ruleLabel(flight.connection) : flight.connection.outcome.label)
                                        .font(.system(size:10,weight:.medium)).foregroundStyle(flight.color)
                                        .opacity(0.9).position(x:p.x,y:p.y-45-(phase.truncatingRemainder(dividingBy:0.2)*100))
                                        .allowsHitTesting(false)
                                    if stage == "internet", let ms = flight.connection.responseMS {
                                        Text("首响应 \(ms) ms").font(.system(size: 10, weight: .medium)).monospacedDigit()
                                            .foregroundStyle(flight.color).position(x: p.x, y: p.y-82).allowsHitTesting(false)
                                    }
                                }
                            }
                        }
                    }
                }.frame(height:350)
                Divider()
                HStack { Text(devices.isEmpty ? "等待设备接入，连接出现后自动播放。" : "真实连接采样回放 · 同域名同色 · 点击图标配置").font(.caption).foregroundStyle(Theme.muted); Spacer(); if model.visibleDevices.count>4 { Button("全部设备") { onSelect(.policy) }.buttonStyle(.plain) } }
            }
        }.onChange(of: model.stats?.uptimeSec) { _ in sample() }.onAppear { sample() }
            .onDisappear { flights=[];feed.reset() }
    }
    private func position(_ n: NetworkNode,width:Double)->CGPoint { CGPoint(x:width*(Double(n.column)+0.5)/7,y:52+n.row*78) }
    private func policyLabel(_ c: ConnectionInfo) -> String {
        switch model.deviceOverride(for: c.srcIP) {
        case "proxy": return "设备指定代理"
        case "direct": return "设备指定直连"
        case "reject": return "设备拒绝"
        default: return "遵循流量规则"
        }
    }
    private func ruleLabel(_ c:ConnectionInfo)->String {
        if c.rejected { return "规则拒绝" }
        if c.fallback { return c.viaProxy ? "二次尝试代理":"降级直连" }
        if c.viaProxy { return "规则代理" }
        return "直连访问"
    }
    private func curve(_ a:CGPoint,_ b:CGPoint)->Path { var p=Path();p.move(to:a);p.addCurve(to:b,control1:CGPoint(x:(a.x+b.x)/2,y:a.y),control2:CGPoint(x:(a.x+b.x)/2,y:b.y));return p }
    private func pointOnCurve(_ a:CGPoint,_ b:CGPoint,t:Double)->CGPoint { let u=1-t,m=(a.x+b.x)/2;return CGPoint(x:u*u*u*a.x+3*u*u*t*m+3*u*t*t*m+t*t*t*b.x,y:u*u*u*a.y+3*u*u*t*a.y+3*u*t*t*b.y+t*t*t*b.y) }
    private func sample() {
        guard let stats=model.stats else { flights=[];feed.reset();return }
        let fresh=feed.ingest(stats.relay.active+stats.relay.recent,uptime:stats.uptimeSec)
        guard !paused && !reducedMotion && motion != "off" else { return }
        flights.removeAll { Date().timeIntervalSince($0.start)>$0.duration }
        let known=Set(nodes.map(\.id))
        for (index,c) in fresh.suffix(8).enumerated() {
            let entry=NetworkIngress.resolve(c,hotspot:network)
            let route=["device:"+c.srcIP,entry.rawValue,entry == .wifi ? "hotspot":"lan","policy","rules",c.rejected ? "reject":c.viaProxy ? "proxy":"direct"]+(c.rejected ? []:["internet"])
            guard route.allSatisfy(known.contains) else { continue }
            flights.append(NetworkFlight(connection:c,path:route,start:Date().addingTimeInterval(Double(index)*0.15)))
        }
        flights=Array(flights.suffix(16))
    }
}
