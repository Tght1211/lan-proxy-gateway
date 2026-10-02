import SwiftUI

extension NetworkFlowHue {
    var color: Color {
        switch self {
        case .green: return .green
        case .yellow: return .yellow
        case .red: return .red
        case .muted: return Theme.muted
        case .purple: return .purple
        case .teal: return .teal
        case .orange: return .orange
        case .pink: return .pink
        case .blue: return .blue
        case .indigo: return .indigo
        case .cyan: return .cyan
        case .mint: return .mint
        }
    }
}

struct NetworkFlowPalettePicker: View {
    @AppStorage("networkFlowPalette") private var palette: NetworkFlowPalette = .status

    var body: some View {
        Menu {
            ForEach(NetworkFlowPalette.allCases) { option in
                Button { palette = option } label: {
                    if option == palette { Label(option.title, systemImage: "checkmark") }
                    else { Text(option.title) }
                }
            }
        } label: {
            HStack(spacing: 7) {
                HStack(spacing: 3) {
                    ForEach(Array(palette.hues.prefix(3).enumerated()), id: \.offset) { _, hue in
                        Circle().fill(hue.color).frame(width: 5, height: 5)
                    }
                }.accessibilityHidden(true)
                Text("流光 · " + palette.title).font(.system(size: 11, weight: .medium))
                Image(systemName: "chevron.down").font(.system(size: 8, weight: .semibold)).foregroundStyle(Theme.muted)
            }.padding(.horizontal, 10).padding(.vertical, 7)
                .background(Theme.panelRaised).clipShape(RoundedRectangle(cornerRadius: 6))
        }
        .menuStyle(.borderlessButton).menuIndicator(.hidden).fixedSize()
        .accessibilityLabel("流光色系").accessibilityValue(palette.title).help(palette.detail)
    }
}
