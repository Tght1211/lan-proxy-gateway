import SwiftUI

struct RuleListPopover: View {
    let title: String
    let color: Color
    let rules: [RoutingRule]
    let onEdit: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack {
                Circle().fill(color).frame(width: 7, height: 7)
                Text("命中「\(title)」的规则").font(.system(size: 12, weight: .semibold))
                Spacer()
                Text("\(rules.count) 条").font(.caption2).foregroundStyle(Theme.muted)
            }
            if rules.isEmpty {
                Text("暂无规则指向此出口，未命中规则的流量走默认出口。")
                    .font(.caption).foregroundStyle(Theme.muted)
                    .fixedSize(horizontal: false, vertical: true)
            } else {
                ScrollView(.vertical, showsIndicators: false) {
                    VStack(alignment: .leading, spacing: 6) {
                        ForEach(rules) { rule in
                            HStack(spacing: 8) {
                                Text(ruleTypeName(rule.type))
                                    .font(.system(size: 9, weight: .bold))
                                    .foregroundStyle(Theme.muted)
                                    .padding(.horizontal, 5).padding(.vertical, 2)
                                    .background(Theme.panelRaised)
                                    .clipShape(RoundedRectangle(cornerRadius: 4))
                                Text(rule.value)
                                    .font(.system(size: 11, design: .monospaced))
                                    .lineLimit(1)
                                if rule.learned {
                                    Text("自动学习")
                                        .font(.system(size: 8, weight: .bold))
                                        .foregroundStyle(Theme.yellow)
                                        .padding(.horizontal, 4).padding(.vertical, 1)
                                        .background(Theme.yellow.opacity(0.12))
                                        .clipShape(RoundedRectangle(cornerRadius: 3))
                                }
                                Spacer(minLength: 0)
                            }
                        }
                    }
                }
                .frame(maxHeight: 180)
            }
            Divider().overlay(Theme.border)
            Button(action: onEdit) {
                Label("编辑分流规则", systemImage: "slider.horizontal.3").font(.caption)
            }
            .buttonStyle(.bordered)
            .controlSize(.small)
        }
        .padding(14)
        .frame(width: 280)
    }

    private func ruleTypeName(_ type: String) -> String {
        switch type {
        case "domain": return "完整域名"
        case "domain-suffix": return "域名后缀"
        case "ip-cidr": return "IP-CIDR"
		case "src-ip": return "设备 IP"
        default: return type
        }
    }
}

struct TopoLinkLayer: View {
    let points: [String: CGPoint]
    let devicePorts: [String]
    let httpDevicePorts: Set<String>
    let gatewayDevicePorts: Set<String>
    let httpActivePorts: Set<String>
    let gatewayActivePorts: Set<String>
    let active: Bool
    let flowProxy: Bool
    let flowDirect: Bool
    let flowReject: Bool
    let egressProxy: Bool
    var hasDeviceOverrides = false
    @State private var isScrolling = false

    var body: some View {
        TimelineView(.animation(minimumInterval: 1.0 / 24.0, paused: !active || isScrolling)) { timeline in
            Canvas { ctx, _ in
                let t = timeline.date.timeIntervalSinceReferenceDate
                let anyFlow = flowProxy || flowDirect
                // Devices → ingress → shared gateway and routing policy.
                for (index, port) in devicePorts.enumerated() {
                    if httpDevicePorts.contains(port) {
                        link(ctx, from: port, to: "ingress.http.b", color: Theme.cyan, phase: t,
                             offset: Double(index) * 0.37, strong: httpActivePorts.contains(port), flow: httpActivePorts.contains(port))
                    }
                    if gatewayDevicePorts.contains(port) {
                        link(ctx, from: port, to: "ingress.gateway.b", color: Theme.lime, phase: t,
                             offset: Double(index) * 0.37, strong: gatewayActivePorts.contains(port), flow: gatewayActivePorts.contains(port))
                    }
                }
                link(ctx, from: "ingress.http.t", to: "gw.b", color: Theme.cyan, phase: t, offset: 0,
                     strong: !httpActivePorts.isEmpty, flow: !httpActivePorts.isEmpty)
                link(ctx, from: "ingress.gateway.t", to: "gw.b", color: Theme.lime, phase: t, offset: 0,
                     strong: !gatewayActivePorts.isEmpty, flow: !gatewayActivePorts.isEmpty)
                // Gateway → ① Device Policy
                link(ctx, from: "gw.t", to: "devpolicy.b", color: Theme.cyan, phase: t, offset: 0.10,
                     strong: anyFlow, flow: anyFlow)
                // ① Device Policy → ② Domain Rules
                link(ctx, from: "devpolicy.t", to: "rules.b", color: hasDeviceOverrides ? Theme.coral : Theme.cyan, phase: t, offset: 0.20,
                     strong: anyFlow, flow: anyFlow)
                // ② Domain Rules → Outcomes
                link(ctx, from: "rules.t", to: "out.proxy", color: Theme.cyan, phase: t, offset: 0.4,
                     strong: egressProxy, flow: flowProxy)
                link(ctx, from: "rules.t", to: "out.direct", color: Theme.lime, phase: t, offset: 0.7,
                     strong: !egressProxy, flow: flowDirect)
                link(ctx, from: "rules.t", to: "out.reject", color: Theme.coral, phase: t, offset: 0.9,
                     strong: false, flow: flowReject)
                // Outcomes → Router
                link(ctx, from: "out.proxy.t", to: "router.b", color: Theme.cyan, phase: t, offset: 0.25,
                     strong: egressProxy, flow: flowProxy)
                link(ctx, from: "out.direct.t", to: "router.b", color: Theme.lime, phase: t, offset: 0.55,
                     strong: !egressProxy, flow: flowDirect)
            }
            .drawingGroup()
        }
        .allowsHitTesting(false)
        .onReceive(NotificationCenter.default.publisher(for: NSScrollView.willStartLiveScrollNotification)) { _ in
            isScrolling = true
        }
        .onReceive(NotificationCenter.default.publisher(for: NSScrollView.didEndLiveScrollNotification)) { _ in
            isScrolling = false
        }
    }

    private func link(_ ctx: GraphicsContext, from: String, to: String, color: Color,
                      phase: Double, offset: Double, strong: Bool, flow: Bool) {
        guard let a = points[from], let b = points[to] else { return }
        let (path, pts) = orthoPath(from: a, to: b)
        ctx.stroke(path, with: .color(color.opacity(strong ? 0.55 : 0.22)),
                   style: StrokeStyle(lineWidth: strong ? 2 : 1.3, lineCap: .round, lineJoin: .round))
        ctx.fill(Path(ellipseIn: CGRect(x: a.x - 2.5, y: a.y - 2.5, width: 5, height: 5)), with: .color(color.opacity(strong ? 0.85 : 0.4)))
        ctx.fill(Path(ellipseIn: CGRect(x: b.x - 2.5, y: b.y - 2.5, width: 5, height: 5)), with: .color(color.opacity(strong ? 0.85 : 0.4)))
        guard active, flow else { return }
        let progress = CGFloat(((phase / 1.7) + offset).truncatingRemainder(dividingBy: 1))
        let dot = point(at: progress, along: pts)
        ctx.fill(Path(ellipseIn: CGRect(x: dot.x - 6, y: dot.y - 6, width: 12, height: 12)), with: .color(color.opacity(0.16)))
        ctx.fill(Path(ellipseIn: CGRect(x: dot.x - 2.8, y: dot.y - 2.8, width: 5.6, height: 5.6)), with: .color(color))
    }

    // Orthogonal routing with rounded elbows: vertical run, shared horizontal
    // bus at the midpoint, vertical run into the target port.
    private func orthoPath(from a: CGPoint, to b: CGPoint) -> (Path, [CGPoint]) {
        var path = Path()
        path.move(to: a)
        if abs(a.x - b.x) < 2 {
            path.addLine(to: b)
            return (path, [a, b])
        }
        let midY = a.y + (b.y - a.y) * 0.5
        let radius = min(7, abs(b.x - a.x) / 2, abs(b.y - a.y) / 4)
        let dirY: CGFloat = b.y > a.y ? 1 : -1
        let dirX: CGFloat = b.x > a.x ? 1 : -1
        path.addLine(to: CGPoint(x: a.x, y: midY - radius * dirY))
        path.addQuadCurve(to: CGPoint(x: a.x + radius * dirX, y: midY),
                          control: CGPoint(x: a.x, y: midY))
        path.addLine(to: CGPoint(x: b.x - radius * dirX, y: midY))
        path.addQuadCurve(to: CGPoint(x: b.x, y: midY + radius * dirY),
                          control: CGPoint(x: b.x, y: midY))
        path.addLine(to: b)
        return (path, [a, CGPoint(x: a.x, y: midY), CGPoint(x: b.x, y: midY), b])
    }

    private func point(at fraction: CGFloat, along pts: [CGPoint]) -> CGPoint {
        var lengths: [CGFloat] = []
        var total: CGFloat = 0
        for i in 1..<pts.count {
            let d = hypot(pts[i].x - pts[i - 1].x, pts[i].y - pts[i - 1].y)
            lengths.append(d)
            total += d
        }
        guard total > 0 else { return pts[0] }
        var remaining = fraction * total
        for i in 1..<pts.count {
            let d = lengths[i - 1]
            if remaining <= d, d > 0 {
                let k = remaining / d
                return CGPoint(x: pts[i - 1].x + (pts[i].x - pts[i - 1].x) * k,
                               y: pts[i - 1].y + (pts[i].y - pts[i - 1].y) * k)
            }
            remaining -= d
        }
        return pts.last ?? .zero
    }
}
