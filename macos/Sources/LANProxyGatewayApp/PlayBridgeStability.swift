import SwiftUI

/// Egress probes are shared with the classic UI; they do not measure console Wi-Fi.
struct PlayBridgeStability: View {
    @EnvironmentObject private var model: AppModel
    private var health: HealthStats? { model.stats?.health }
    private var hasSamples: Bool { health?.history.isEmpty == false }
    private var successes: Int { health?.history.filter(\.ok).count ?? 0 }
    private var fresh: Bool {
        guard let checkedAt = health?.checkedAt else { return false }
        return Date().timeIntervalSince(checkedAt) <= 30
    }
    private var status: String {
        guard hasSamples else { return "等待探测" }
        guard fresh else { return "探测已暂停" }
        return health?.healthy == true ? "最近探测成功" : "最近探测失败"
    }
    private var statusColor: Color {
        guard hasSamples && fresh else { return .secondary }
        return health?.healthy == true ? Theme.lime : Theme.coral
    }

    var body: some View {
        Panel(fillHeight: true) {
            VStack(alignment: .leading, spacing: 16) {
                HStack {
                    Text("稳定性").font(.headline)
                    Spacer()
                    Label(status, systemImage: "circle.fill")
                        .font(.caption2).foregroundStyle(statusColor)
                }
                HStack(alignment: .firstTextBaseline, spacing: 8) {
                    Text(hasSamples ? String(format: "%.1f%%", health?.availability ?? 0) : "—")
                        .font(.system(size: 28, weight: .semibold, design: .rounded)).monospacedDigit()
                    Text("探测可用率").font(.caption).foregroundStyle(.secondary)
                }
                HStack(spacing: 24) {
                    metric("平均延迟", successes > 0 ? formatMS(health?.latencyMS) : "—", Theme.cyan)
                    metric("平均抖动", successes > 1 ? formatMS(health?.jitterMS) : "—", Theme.yellow)
                    Spacer(minLength: 0)
                }
                ProbeHistoryStrip(points: Array((health?.history ?? []).suffix(60)),
                                  average: health?.latencyMS ?? 0)
                HStack(spacing: 10) {
                    legend("正常", Theme.lime)
                    legend("延迟偏高", Theme.yellow)
                    legend("失败", Theme.coral)
                    legend("无数据", Theme.border)
                }
                Text("Mac 默认出口探测，不代表主机 Wi-Fi 质量。指标汇总最近最多 180 次探测；条带显示最近 60 次（约 10 分钟）。")
                    .font(.caption2).foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
    }

    private func metric(_ title: String, _ value: String, _ color: Color) -> some View {
        VStack(alignment: .leading, spacing: 5) {
            Text(title).font(.caption).foregroundStyle(.secondary)
            Text(value).font(.system(size: 16, weight: .semibold, design: .rounded))
                .monospacedDigit().foregroundStyle(color)
        }
    }
    private func legend(_ title: String, _ color: Color) -> some View {
        HStack(spacing: 4) {
            Circle().fill(color).frame(width: 5, height: 5)
            Text(title).font(.system(size: 10)).foregroundStyle(.secondary)
        }
    }
}
