import SwiftUI

struct DevicePolicyPopover: View {
    @EnvironmentObject private var model: AppModel

    private var allDevices: [UsageAggregate] { model.stats?.relay.devices ?? [] }

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            // Header
            HStack(spacing: 10) {
                ZStack {
                    RoundedRectangle(cornerRadius: Theme.radius).fill(Theme.coral.opacity(0.14))
                    Image(systemName: "person.crop.circle.badge.checkmark")
                        .font(.system(size: 14, weight: .semibold)).foregroundStyle(Theme.coral)
                }
                .frame(width: 30, height: 30)
                VStack(alignment: .leading, spacing: 1) {
                    Text("① 设备策略").font(.system(size: 13, weight: .semibold))
                    Text("设备级前置阀门，优先于域名规则")
                        .font(.system(size: 10)).foregroundStyle(Theme.muted)
                }
                Spacer()
            }
            .padding(14)

            // Flow explanation
            HStack(spacing: 0) {
                FlowStep(icon: "desktopcomputer", text: "设备流量", color: Theme.lime)
                FlowArrow()
                FlowStep(icon: "person.crop.circle.badge.checkmark", text: "设备策略", color: Theme.coral)
                FlowArrow()
                FlowStep(icon: "arrow.triangle.branch", text: "域名规则", color: Theme.yellow)
            }
            .padding(.horizontal, 14).padding(.bottom, 12)

            Divider().overlay(Theme.border)

            // Device list
            VStack(alignment: .leading, spacing: 10) {
                if allDevices.isEmpty {
                    Label("暂无设备接入", systemImage: "antenna.radiowaves.left.and.right.slash")
                        .font(.caption).foregroundStyle(Theme.muted)
                } else {
                    Text("当前设备 · \(allDevices.count)").eyebrow()
                    ForEach(allDevices.prefix(10)) { device in
                        DevicePolicyRow(ip: device.name)
                    }
                    if allDevices.count > 10 {
                        Text("还有 \(allDevices.count - 10) 台设备…")
                            .font(.caption2).foregroundStyle(Theme.muted)
                    }
                }
            }
            .padding(14)
        }
        .frame(width: 340)
    }
}

struct DevicePolicyRow: View {
    @EnvironmentObject private var model: AppModel
    let ip: String

    private var override: String { model.deviceOverride(for: ip) }
    private var adaptive: DeviceAdaptiveState? { model.adaptiveDeviceState(for: ip) }
    private var label: String? { model.effectiveDeviceLabel(for: ip).nonEmpty }
    private var isProtected: Bool { override.isEmpty && adaptive?.mode == "direct" }

    var body: some View {
        HStack(spacing: 8) {
            Image(systemName: deviceIcon(label: label))
                .font(.system(size: 11, weight: .semibold))
                .foregroundStyle(statusColor)
                .frame(width: 18)
            VStack(alignment: .leading, spacing: 1) {
                Text(label ?? ip)
                    .font(.system(size: 11, weight: .semibold, design: (label != nil) ? .default : .monospaced))
                    .lineLimit(1)
                if label != nil {
                    Text(ip).font(.system(size: 9, design: .monospaced)).foregroundStyle(Theme.muted).lineLimit(1)
                }
            }
            Spacer(minLength: 4)
            if isProtected {
                HStack(spacing: 3) {
                    Image(systemName: "shield.fill").font(.system(size: 8))
                    Text("保护直连").font(.system(size: 10, weight: .medium))
                }
                .foregroundStyle(Theme.yellow)
                .padding(.horizontal, 6).padding(.vertical, 2)
                .background(Theme.yellow.opacity(0.12)).clipShape(Capsule())
            } else {
                Menu {
                    Button { model.setDeviceOverride(ip, action: "") } label: {
                        Label("默认（跟随规则）", systemImage: override.isEmpty ? "checkmark" : "")
                    }
                    Button { model.setDeviceOverride(ip, action: "proxy") } label: {
                        Label("强制代理", systemImage: override == "proxy" ? "checkmark" : "")
                    }
                    Button { model.setDeviceOverride(ip, action: "direct") } label: {
                        Label("强制直连", systemImage: override == "direct" ? "checkmark" : "")
                    }
                    Divider()
                    Button { model.setDeviceOverride(ip, action: "reject") } label: {
                        Label("拒绝联网", systemImage: override == "reject" ? "checkmark" : "")
                    }
                } label: {
                    HStack(spacing: 3) {
                        Circle().fill(statusColor).frame(width: 6, height: 6)
                        Text(statusText).font(.system(size: 10, weight: .medium))
                        Image(systemName: "chevron.up.chevron.down").font(.system(size: 7))
                    }
                    .foregroundStyle(statusColor == Theme.muted ? .primary : statusColor)
                    .padding(.horizontal, 6).padding(.vertical, 2)
                    .background(statusColor.opacity(0.10)).clipShape(Capsule())
                }
                .menuStyle(.borderlessButton)
                .menuIndicator(.hidden)
                .fixedSize()
            }
        }
    }

    private var statusColor: Color {
        if isProtected { return Theme.yellow }
        switch override {
        case "proxy": return Theme.cyan
        case "direct": return Theme.lime
        case "reject": return Theme.coral
        default: return Theme.muted
        }
    }

    private var statusText: String {
        if isProtected { return "保护直连" }
        switch override {
        case "proxy": return "代理"
        case "direct": return "直连"
        case "reject": return "拒绝"
        default: return "默认"
        }
    }
}

func deviceIcon(label: String?) -> String {
    guard let label = label?.lowercased() else { return "desktopcomputer" }
    let phones = ["iphone", "手机", "phone", "一加", "oneplus", "xiaomi", "小米", "huawei", "华为", "oppo", "vivo", "pixel", "安卓", "android"]
    let consoles = ["switch", "playstation", "ps5", "ps4", "xbox", "游戏", "主机"]
    let tvs = ["tv", "电视", "盒子"]
    let pads = ["ipad", "pad", "平板"]
    let laptops = ["mac", "笔记本", "电脑", "pc", "laptop"]
    if phones.contains(where: { label.contains($0) }) { return "iphone" }
    if consoles.contains(where: { label.contains($0) }) { return "gamecontroller.fill" }
    if tvs.contains(where: { label.contains($0) }) { return "appletv.fill" }
    if pads.contains(where: { label.contains($0) }) { return "ipad.landscape" }
    if laptops.contains(where: { label.contains($0) }) { return "laptopcomputer" }
    return "desktopcomputer"
}

struct TopoDeviceChip: View {
    let icon: String
    let title: String
    let subtitle: String
    let active: Bool

    var body: some View {
        HStack(spacing: 8) {
            ZStack {
                RoundedRectangle(cornerRadius: Theme.radiusSmall).fill((active ? Theme.lime : Theme.muted).opacity(0.12))
                Image(systemName: icon)
                    .font(.system(size: 11, weight: .semibold))
                    .foregroundStyle(active ? Theme.lime : Theme.muted)
            }
            .frame(width: 24, height: 24)
            VStack(alignment: .leading, spacing: 1) {
                Text(title)
                    .font(.system(size: 11, weight: .semibold, design: title.contains(".") ? .monospaced : .default))
                    .lineLimit(1)
                Text(subtitle)
                    .font(.system(size: 9, design: .monospaced)).foregroundStyle(Theme.muted).lineLimit(1)
            }
        }
        .padding(.horizontal, 10).frame(height: 40)
        .background(Theme.panel)
        .overlay(RoundedRectangle(cornerRadius: Theme.radius).stroke(active ? Theme.lime.opacity(0.35) : Theme.border, lineWidth: Theme.borderWidth))
        .clipShape(RoundedRectangle(cornerRadius: Theme.radius))
    }
}

struct TopoStage: View {
    let portID: String
    let icon: String
    let tint: Color
    let title: String
    let detail: String
    var clickable = false
    @State private var hovering = false

    var body: some View {
        HStack(spacing: 11) {
            ZStack {
                RoundedRectangle(cornerRadius: 10).fill(tint.opacity(hovering && clickable ? 0.17 : 0.10))
                Image(systemName: icon).font(.system(size: 16, weight: .semibold)).foregroundStyle(tint)
            }
            .frame(width: 42, height: 42)
            .overlay(RoundedRectangle(cornerRadius: 10).stroke(hovering && clickable ? tint.opacity(0.6) : Theme.border, lineWidth: 1))
            .overlay(alignment: .bottomTrailing) {
                if clickable {
                    Image(systemName: "pencil")
                        .font(.system(size: 7, weight: .bold)).foregroundStyle(Color.white)
                        .frame(width: 14, height: 14)
                        .background(tint).clipShape(Circle())
                        .offset(x: 4, y: 4)
                }
            }
            VStack(alignment: .leading, spacing: 2) {
                Text(title).font(.system(size: 12, weight: .semibold))
                Text(detail)
                    .font(.system(size: 10, design: .monospaced)).foregroundStyle(Theme.muted)
                    .lineLimit(1).minimumScaleFactor(0.8)
            }
        }
        .padding(.horizontal, 13).frame(height: 58)
        .background(Theme.panel)
        .overlay(RoundedRectangle(cornerRadius: 9).stroke(hovering && clickable ? tint.opacity(0.5) : Theme.border, lineWidth: 0.8))
        .clipShape(RoundedRectangle(cornerRadius: 9))
        .topoPort("\(portID).t", .top)
        .topoPort("\(portID).b", .bottom)
        .onHover { hovering = $0 }
    }
}

struct TopoOutcome: View {
    let icon: String
    let title: String
    let detail: String
    let count: Int
    let color: Color
    let emphasized: Bool
    var badge: String? = nil
    var badgeColor: Color? = nil
    var breathing = false
    var help = "点击配置网络出口"
    var ruleList: [RoutingRule]? = nil
    var onEditRules: (() -> Void)? = nil
    let action: () -> Void
    @State private var hovering = false
    @State private var showRules = false
    @State private var pulsing = false

    var body: some View {
        Button {
            if ruleList != nil {
                showRules.toggle()
            } else {
                action()
            }
        } label: {
            VStack(spacing: 3) {
                ZStack {
                    RoundedRectangle(cornerRadius: Theme.radiusSmall).fill(color.opacity(0.12))
                    Image(systemName: icon).font(.system(size: 11, weight: .semibold)).foregroundStyle(color)
                }
                .frame(width: 24, height: 24)
                HStack(spacing: 4) {
                    Text(title).font(.system(size: 11, weight: .semibold))
                    if let badge {
                        Text(badge)
                            .font(.system(size: 8, weight: .bold)).foregroundStyle(badgeColor ?? color)
                            .padding(.horizontal, 4).padding(.vertical, 1)
                            .background((badgeColor ?? color).opacity(0.12)).clipShape(Capsule())
                    }
                }
                Text(detail)
                    .font(.system(size: 9, design: .monospaced)).foregroundStyle(Theme.muted)
                    .lineLimit(1).minimumScaleFactor(0.75)
            }
            .padding(.horizontal, 9).padding(.vertical, 8)
            .frame(width: 148)
            .overlay(alignment: .topTrailing) {
                Text("\(count)")
                    .font(.system(size: 9, weight: .bold, design: .monospaced))
                    .foregroundStyle(count > 0 ? Color.white : Theme.muted)
                    .frame(minWidth: 16, minHeight: 16)
                    .background(count > 0 ? color : Theme.panelRaised)
                    .clipShape(Circle())
                    .offset(x: -5, y: 5)
                    .help("\(count) 条规则指向此出口")
            }
            .background(Theme.panel)
            .overlay(
                RoundedRectangle(cornerRadius: 8)
                    .stroke(strokeColor, lineWidth: emphasized ? 1.2 : 0.8)
            )
            .clipShape(RoundedRectangle(cornerRadius: 8))
            .shadow(color: breathing ? color.opacity(pulsing ? 0.30 : 0.05) : (emphasized ? color.opacity(0.10) : .clear),
                    radius: breathing ? (pulsing ? 9 : 3) : 6, y: 1)
        }
        .buttonStyle(.plain)
        .onHover { hovering = $0 }
        .help(help)
        .onAppear {
            guard breathing else { return }
            withAnimation(.easeInOut(duration: 1.5).repeatForever(autoreverses: true)) {
                pulsing = true
            }
        }
        .popover(isPresented: $showRules, arrowEdge: .bottom) {
            RuleListPopover(title: title, color: color, rules: ruleList ?? []) {
                showRules = false
                onEditRules?()
            }
        }
    }

    private var strokeColor: Color {
        if hovering { return color.opacity(0.65) }
        if breathing { return color.opacity(pulsing ? 0.65 : 0.30) }
        if emphasized { return color.opacity(0.45) }
        return Theme.border
    }
}
