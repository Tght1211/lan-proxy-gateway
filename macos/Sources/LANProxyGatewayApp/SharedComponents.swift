import AppKit
import Charts
import SwiftUI
import UniformTypeIdentifiers

// MARK: - Components

struct Panel<Content: View>: View {
    @ViewBuilder let content: Content
    var body: some View {
        VStack(alignment: .leading, spacing: 0) { content }
            .padding(16)
            .frame(maxWidth: .infinity, alignment: .topLeading)
            .background(Theme.panel)
            .overlay(RoundedRectangle(cornerRadius: Theme.radius).stroke(Theme.border, lineWidth: Theme.borderWidth))
            .clipShape(RoundedRectangle(cornerRadius: Theme.radius))
            .shadow(color: Color.black.opacity(Theme.shadowOpacity), radius: Theme.shadowRadius, y: 2)
    }
}

struct MetricCard: View {
    let label: String, value: String, icon: String
    let color: Color
    init(_ label: String, _ value: String, _ icon: String, _ color: Color) { self.label = label; self.value = value; self.icon = icon; self.color = color }
    var body: some View {
        HStack(spacing: 11) {
            ZStack { Circle().fill(color.opacity(0.11)); Image(systemName: icon).foregroundStyle(color).font(.system(size: 15, weight: .semibold)) }.frame(width: 34, height: 34)
            VStack(alignment: .leading, spacing: 3) { Text(label).font(.caption).foregroundStyle(Theme.muted); Text(value).font(.system(size: 18, weight: .semibold, design: .rounded)).lineLimit(1).minimumScaleFactor(0.7) }
            Spacer(minLength: 0)
        }.padding(.horizontal, 13).frame(minHeight: 66).background(Theme.panel).overlay(RoundedRectangle(cornerRadius: Theme.radius).stroke(Theme.border, lineWidth: Theme.borderWidth)).clipShape(RoundedRectangle(cornerRadius: Theme.radius))
    }
}

struct LiveBadge: View {
    let active: Bool
    var body: some View { HStack(spacing: 6) { Circle().fill(active ? Theme.lime : Theme.coral).frame(width: 6, height: 6); Text(active ? "正常" : "异常") }.font(.system(size: 10, weight: .semibold)).foregroundStyle(active ? Theme.lime : Theme.coral).padding(.horizontal, 8).frame(height: 24).background((active ? Theme.lime : Theme.coral).opacity(0.09)).clipShape(RoundedRectangle(cornerRadius: 5)) }
}

struct ValuePair: View {
    let label: String, value: String
    var body: some View { VStack(alignment: .leading, spacing: 3) { Text(label.uppercased()).font(.system(size: 8, weight: .bold)).foregroundStyle(Theme.muted); Text(value).font(.system(size: 12, weight: .semibold, design: .monospaced)).lineLimit(1) } }
}

struct ChartLegend: View {
    let color: Color, text: String
    var body: some View { HStack(spacing: 5) { Capsule().fill(color).frame(width: 13, height: 3); Text(text).font(.caption2).foregroundStyle(Theme.muted) } }
}

struct RecentStrip: View {
    @EnvironmentObject private var model: AppModel
    var body: some View {
        Panel {
            VStack(alignment: .leading, spacing: 12) {
                HStack { Text("最近连接").sectionLabel(); Spacer(); Button("查看全部") { model.selectedSection = .connections }.buttonStyle(.plain).foregroundStyle(Theme.cyan).font(.caption) }
                ForEach(Array((model.stats?.relay.recent ?? []).prefix(5))) { item in
                    HStack(spacing: 12) {
                        Circle().fill(serviceColor(item.service)).frame(width: 7, height: 7)
                        HStack(spacing: 4) {
                            Text(item.service).fontWeight(.medium)
                            if item.isUDP {
                                Text("UDP").font(.system(size: 7, weight: .bold, design: .monospaced))
                                    .padding(.horizontal, 2).padding(.vertical, 1)
                                    .background(Theme.yellow.opacity(0.15))
                                    .foregroundStyle(Theme.yellow)
                                    .clipShape(RoundedRectangle(cornerRadius: 2))
                            }
                        }.frame(width: 125, alignment: .leading)
                        Text(item.dstHost).font(.system(.caption, design: .monospaced)).foregroundStyle(Theme.muted).lineLimit(1)
                        Spacer()
                        Text(item.srcIP).font(.system(.caption, design: .monospaced)).foregroundStyle(Theme.muted)
                        Text(bytes(item.up + item.down)).font(.caption).frame(width: 70, alignment: .trailing)
                    }.frame(height: 25)
                }
                if model.stats?.relay.recent.isEmpty != false { EmptyTelemetry(icon: "clock", text: "等待访问记录") }
            }
        }
    }
}

struct EmptyTelemetry: View {
    let icon: String, text: String
    var body: some View { VStack(spacing: 9) { Image(systemName: icon).font(.title2).foregroundStyle(Theme.muted); Text(text).font(.caption).foregroundStyle(Theme.muted) }.frame(maxWidth: .infinity, maxHeight: .infinity).padding(24) }
}

struct SetupValue: View {
    let label: String, value: String
    init(_ label: String, _ value: String) { self.label = label; self.value = value }
    var body: some View { VStack(alignment: .leading, spacing: 6) { Text(label).font(.caption).foregroundStyle(Theme.muted); HStack(spacing: 7) { Text(value).font(.system(.body, design: .monospaced)); Button { NSPasteboard.general.clearContents(); NSPasteboard.general.setString(value, forType: .string) } label: { Image(systemName: "doc.on.doc") }.buttonStyle(.plain).foregroundStyle(Theme.cyan) } } }
}

struct QualityRow: View {
    let label: String, value: String, color: Color
    init(_ label: String, _ value: String, _ color: Color) { self.label = label; self.value = value; self.color = color }
    var body: some View { HStack { Text(label).foregroundStyle(Theme.muted); Spacer(); Text(value).foregroundStyle(color).fontWeight(.semibold) }.font(.caption) }
}

struct ExitFact: View {
    let label: String, value: String, icon: String
    var body: some View {
        HStack(spacing: 10) {
            Image(systemName: icon).foregroundStyle(Theme.cyan).frame(width: 22)
            VStack(alignment: .leading, spacing: 3) {
                Text(label).font(.caption2).foregroundStyle(Theme.muted)
                Text(value).font(.system(size: 12, weight: .semibold, design: .rounded))
                    .lineLimit(1).minimumScaleFactor(0.72)
            }
            Spacer(minLength: 8)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, 10)
    }
}

struct SettingsRow<Actions: View>: View {
    let title: String, detail: String
    @ViewBuilder let actions: Actions
    var body: some View { HStack { VStack(alignment: .leading, spacing: 4) { Text(title).fontWeight(.semibold); Text(detail).font(.caption).foregroundStyle(Theme.muted).lineLimit(1) }; Spacer(); HStack { actions } }.padding(.vertical, 10) }
}
