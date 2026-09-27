import AppKit
import Charts
import SwiftUI
import UniformTypeIdentifiers

struct DevicesView: View {
    @EnvironmentObject private var model: AppModel
    @State private var showOnboarding = false

    var body: some View {
        ScrollPage {
            DeviceAccessSummary { showOnboarding = true }
            ProxyUsageOverview()
            LabeledDevicesStrip()
            NATDiagPanel()
            HStack(alignment: .top, spacing: 16) {
                DeviceRanking().frame(maxWidth: .infinity)
                ServiceUsagePanel()
                    .frame(minWidth: 280, idealWidth: 360, maxWidth: 380)
                    .frame(height: 460)
            }
        }
        .sheet(isPresented: $showOnboarding) {
            DeviceOnboardingSheet().environmentObject(model)
        }
    }
}

struct LabeledDevicesStrip: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        let labels = model.deviceLabels.sorted { $0.key < $1.key }
        if !labels.isEmpty {
            HStack(spacing: 10) {
                Text("已备注设备").sectionLabel()
                ForEach(labels, id: \.key) { ip, label in
                    HStack(spacing: 7) {
                        Image(systemName: "tag.fill").foregroundStyle(Theme.cyan)
                        Text(label).font(.caption.weight(.semibold))
                        Text(ip).font(.system(size: 10, design: .monospaced)).foregroundStyle(Theme.muted)
                        Button { model.setDeviceLabel("", for: ip) } label: { Image(systemName: "xmark") }
                            .buttonStyle(.plain).foregroundStyle(Theme.muted).help("移除备注")
                    }
                    .padding(.horizontal, 10).frame(height: 30)
                    .background(Theme.panel)
                    .overlay(RoundedRectangle(cornerRadius: Theme.radiusSmall).stroke(Theme.border, lineWidth: Theme.borderWidth))
                    .clipShape(RoundedRectangle(cornerRadius: Theme.radiusSmall))
                }
                Spacer()
            }
        }
    }
}

// MARK: - NAT Diagnosis Panel

struct NATDiagPanel: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        Panel {
            VStack(alignment: .leading, spacing: 10) {
                HStack {
                    VStack(alignment: .leading, spacing: 2) {
                        Text("NAT 环境诊断").font(.system(size: 13, weight: .semibold))
                        Text("检测 NAT 类型、双重 NAT 和 UPnP 状态，帮助排查游戏联机问题")
                            .font(.caption2).foregroundStyle(Theme.muted)
                    }
                    Spacer()
                    Button { model.runNATDiag() } label: {
                        if model.isNATDiagRunning {
                            HStack(spacing: 6) {
                                ProgressView().controlSize(.small)
                                Text("诊断中…")
                            }
                        } else {
                            Label("开始诊断", systemImage: "network.badge.shield.half.filled")
                        }
                    }
                    .buttonStyle(.bordered).controlSize(.small)
                    .disabled(model.isNATDiagRunning || !model.isRunning)
                }
                if let diag = model.natDiag {
                    HStack(spacing: 20) {
                        NATFactChip(label: "NAT 类型", value: natTypeLabel(diag.natType),
                                    color: natTypeColor(diag.natType), icon: "shield.checkerboard")
                        NATFactChip(label: "公网 IP", value: diag.externalIP.isEmpty ? "--" : diag.externalIP,
                                    color: Theme.cyan, icon: "globe")
                        NATFactChip(label: "UPnP", value: diag.upnp.available ? "已启用" : "未检测到",
                                    color: diag.upnp.available ? Theme.lime : Theme.yellow,
                                    icon: diag.upnp.available ? "checkmark.circle" : "exclamationmark.triangle")
                        NATFactChip(label: "双重 NAT", value: diag.doubleNAT ? "是" : "否",
                                    color: diag.doubleNAT ? Theme.coral : Theme.lime,
                                    icon: diag.doubleNAT ? "exclamationmark.triangle.fill" : "checkmark.circle")
                        Spacer()
                    }
                    if let warnings = diag.warnings, !warnings.isEmpty {
                        VStack(alignment: .leading, spacing: 4) {
                            ForEach(Array(warnings.enumerated()), id: \.offset) { _, warning in
                                HStack(spacing: 6) {
                                    Image(systemName: "exclamationmark.triangle.fill")
                                        .font(.caption2).foregroundStyle(Theme.yellow)
                                    Text(warning).font(.caption2).foregroundStyle(Theme.muted)
                                }
                            }
                        }
                    }
                    if let device = diag.upnp.deviceName, !device.isEmpty {
                        HStack(spacing: 4) {
                            Text("UPnP 设备:").font(.caption2).foregroundStyle(Theme.muted)
                            Text(device).font(.caption2.monospaced()).foregroundStyle(Theme.muted)
                        }
                    }
                }
            }
        }
    }

    private func natTypeLabel(_ type: String) -> String {
        switch type {
        case "open": return "Open (NAT 1)"
        case "moderate": return "Moderate (NAT 2)"
        case "strict": return "Strict (NAT 3)"
        default: return type
        }
    }

    private func natTypeColor(_ type: String) -> Color {
        switch type {
        case "open": return Theme.lime
        case "moderate": return Theme.yellow
        case "strict": return Theme.coral
        default: return Theme.muted
        }
    }
}

struct NATFactChip: View {
    let label: String
    let value: String
    let color: Color
    let icon: String

    var body: some View {
        HStack(spacing: 6) {
            Image(systemName: icon).font(.caption).foregroundStyle(color)
            VStack(alignment: .leading, spacing: 1) {
                Text(label).font(.system(size: 9)).foregroundStyle(Theme.muted)
                Text(value).font(.system(size: 11, weight: .medium)).foregroundStyle(color)
            }
        }
        .padding(.horizontal, 10).padding(.vertical, 6)
        .background(color.opacity(0.08))
        .clipShape(RoundedRectangle(cornerRadius: Theme.radiusSmall))
    }
}

struct DeviceAccessSummary: View {
    @EnvironmentObject private var model: AppModel
    let onOnboard: () -> Void

    init(onOnboard: @escaping () -> Void) { self.onOnboard = onOnboard }

    var body: some View {
        Panel {
            HStack(spacing: 20) {
                ZStack {
                    RoundedRectangle(cornerRadius: Theme.radius).fill(Theme.lime.opacity(0.12))
                    Image(systemName: "desktopcomputer.and.macbook")
                        .font(.system(size: 22, weight: .medium)).foregroundStyle(Theme.lime)
                }
                .frame(width: 52, height: 52)
                VStack(alignment: .leading, spacing: 4) {
                    Text("\(model.stats?.relay.devices.count ?? 0) 台设备已接入")
                        .font(.system(size: 16, weight: .semibold))
                        .lineLimit(1).fixedSize(horizontal: true, vertical: false)
                    Text("其中 \(model.activeDeviceCount) 台正在产生连接")
                        .font(.caption).foregroundStyle(Theme.muted)
                        .lineLimit(1).fixedSize(horizontal: true, vertical: false)
                }
                Spacer()
                CompactSetupValue(label: "活动连接", value: "\(model.stats?.relay.active.count ?? 0)")
                CompactSetupValue(label: "已识别服务", value: "\(model.stats?.relay.services.count ?? 0)")
                if model.status?.accessMode == "hotspot" {
                    CompactSetupValue(label: "设备地址", value: "自动分配")
                    CompactSetupValue(label: "代理 Wi-Fi", value: model.stats?.hotspot?.applied == true ? "接管已开启" : "等待接管")
                } else {
                    CompactSetupValue(label: "网关与 DNS", value: model.status?.gateway.localIP.nonEmpty ?? "--", copyable: true)
                    CompactSetupValue(label: "子网掩码", value: "255.255.255.0", copyable: true)
                }
                Button(action: onOnboard) {
                    Label("接入设备", systemImage: "plus.circle.fill")
                }
                .buttonStyle(ActionButtonStyle(tint: Theme.cyan))
                .help("让 Switch / PS5 连接代理 Wi-Fi")
            }
        }
    }
}
