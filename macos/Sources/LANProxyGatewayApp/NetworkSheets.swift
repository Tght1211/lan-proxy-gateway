import SwiftUI
import AppKit

enum NetworkSheet: Identifiable {
    case device(String), access(String), gateway(String), policy, rules, learning, proxy, direct, reject, internet
    var id: String {
        switch self {
        case .device(let value): return "device:" + value
        case .access(let value): return "access:" + value
        case .gateway(let value): return "gateway:" + value
        case .policy: return "policy"
        case .rules: return "rules"
        case .learning: return "learning"
        case .proxy: return "proxy"
        case .direct: return "direct"
        case .reject: return "reject"
        case .internet: return "internet"
        }
    }
}

/// Every topology intent opens a sheet, never redirects the underlying page.
struct NetworkSheetContent: View {
    @EnvironmentObject private var model: AppModel
    @Environment(\.dismiss) private var dismiss
    let destination: NetworkSheet
    var body: some View {
        switch destination {
        case .proxy: ProxyConfigSheet()
        case .rules: RoutingRulesEditor(rules: model.status?.routing ?? [])
        case .device(let ip):
            DeviceTrafficSheet(device: ip, showsPolicy: true)
        case .access(let method):
            if method == "wifi" { WiFiGuideView() }
            else { SheetFrame(title: "设备接入") { NetworkAccessInstructions(method: method) } }
        case .gateway(let address):
            SheetFrame(title: "Mac 网络入口", sizing: .information) {
                SetupValue("入口地址", address)
                Text("同一台 Mac 的不同网络接口，共用设备策略与流量规则。")
                    .foregroundStyle(Theme.muted)
                Text(model.hotspotControlDetail).font(.caption).foregroundStyle(Theme.muted)
            }
        case .policy: SheetFrame(title: "设备策略", sizing: .policy) { DevicePolicyPopover() }
        case .learning: SheetFrame(title: "自学习", sizing: .workspace) { NetworkLearningView() }
        case .direct: DeviceTrafficSheet(device: "", initialRoute: "direct")
        case .reject:
            SheetFrame(title: "拒绝记录", sizing: .records) {
                ForEach((model.status?.routing ?? []).filter { $0.action == "reject" }) { rule in
                    HStack { Text(rule.value).textSelection(.enabled); Spacer(); Text(rule.type).foregroundStyle(Theme.muted) }
                }
                Divider()
                ForEach(Array((model.stats?.relay.recent ?? []).filter(\.rejected).prefix(20))) { row in
                    HStack { Text(row.srcIP); Text(row.dstHost); Spacer(); Text("已拒绝").foregroundStyle(Theme.coral) }.font(.caption)
                }
                if !(model.stats?.relay.recent.contains(where: \.rejected) ?? false) { Text("暂无最近拒绝记录").foregroundStyle(Theme.muted) }
            }
        case .internet:
            SheetFrame(title: "出口与目标服务", sizing: .services) {
                if let identity = model.stats?.health.egressIdentity { SetupValue("探测出口 IP", identity.ip) }
                Text("响应状态来自核心记录；收到数据不代表业务操作成功。")
                    .font(.caption).foregroundStyle(Theme.muted)
                ServiceUsagePanel().frame(height: 320)
            }
        }
    }
}

private struct SheetFrameMeasurements: Equatable {
    var header: CGFloat = 0
    var content: CGFloat = 0
}

private struct SheetFrameMeasurementsKey: PreferenceKey {
    static let defaultValue = SheetFrameMeasurements()

    static func reduce(value: inout SheetFrameMeasurements, nextValue: () -> SheetFrameMeasurements) {
        let next = nextValue()
        value.header = max(value.header, next.header)
        value.content = max(value.content, next.content)
    }
}

struct SheetFrame<Content: View>: View {
    @Environment(\.dismiss) private var dismiss
    @State private var measurements = SheetFrameMeasurements(header: 64)
    let title: String
    var sizing: ContentSheetSizing = .access
    @ViewBuilder let content: Content
    var body: some View {
        let screenSize = NSApp.keyWindow?.screen?.visibleFrame.size ?? NSScreen.main?.visibleFrame.size ?? CGSize(width: 1440, height: 900)
        let size = sizing.fittedSize(contentHeight: measurements.content, headerHeight: measurements.header,
                                     availableWidth: screenSize.width - 80, availableHeight: screenSize.height - 120)
        VStack(spacing: 0) {
            HStack { Text(title).font(.title3.weight(.semibold)); Spacer(); Button { dismiss() } label: { Image(systemName: "xmark") }.buttonStyle(.plain).accessibilityLabel("关闭") }.padding(22)
                .fixedSize(horizontal: false, vertical: true)
                .background(GeometryReader { geometry in
                    Color.clear.preference(key: SheetFrameMeasurementsKey.self, value: SheetFrameMeasurements(header: geometry.size.height))
                })
            Divider()
            ScrollView {
                VStack(alignment: .leading, spacing: 18) { content }
                    .padding(22).frame(maxWidth: .infinity, alignment: .leading)
                    .fixedSize(horizontal: false, vertical: true)
                    .background(GeometryReader { geometry in
                        Color.clear.preference(key: SheetFrameMeasurementsKey.self, value: SheetFrameMeasurements(content: geometry.size.height))
                    })
            }
        }.frame(width: size.width, height: size.height)
            .onPreferenceChange(SheetFrameMeasurementsKey.self) { measured in
                if measurements != measured { measurements = measured }
            }
            .foregroundStyle(Theme.text).buttonStyle(StudioButtonStyle()).background(Theme.panel)
    }
}
