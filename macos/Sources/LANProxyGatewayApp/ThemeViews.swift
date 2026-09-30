import AppKit
import Charts
import SwiftUI
import UniformTypeIdentifiers

// MARK: - Theme Selector with Import/Export

struct ThemeSelectorPanel: View {
    @EnvironmentObject private var model: AppModel
    @State private var customThemes: [ThemePalette] = CustomThemeStore.loadAll()
    @State private var importError: String?
    @State private var showImportError = false
    @State private var showDeleteConfirm = false
    @State private var pendingDeleteID: String?

    private var allThemes: [ThemePalette] { ThemePalette.builtIn + customThemes }

    var body: some View {
        Panel {
            VStack(alignment: .leading, spacing: 12) {
                HStack {
                    VStack(alignment: .leading, spacing: 2) {
                        Text("外观主题").font(.system(size: 13, weight: .semibold))
                        Text("即时生效，自动记住选择").font(.caption2).foregroundStyle(Theme.muted)
                    }
                    Spacer()
                    HStack(spacing: 6) {
                        Button { importTheme() } label: {
                            Label("导入", systemImage: "square.and.arrow.down")
                        }
                        .buttonStyle(StudioButtonStyle()).controlSize(.small)
                        .help("从 JSON 文件导入主题")

                        Button { exportCurrent() } label: {
                            Label("导出", systemImage: "square.and.arrow.up")
                        }
                        .buttonStyle(StudioButtonStyle()).controlSize(.small)
                        .help("将当前主题导出为 JSON 文件")
                    }
                }
                ScrollView(.horizontal, showsIndicators: false) {
                    HStack(spacing: 12) {
                        ForEach(allThemes) { palette in
                            ThemeCard(palette: palette, selected: model.themeID == palette.id) {
                                model.themeID = palette.id
                            }
                            .contextMenu {
                                if !palette.isBuiltIn {
                                    Button { exportTheme(palette) } label: { Label("导出此主题", systemImage: "square.and.arrow.up") }
                                    Divider()
                                    Button(role: .destructive) {
                                        pendingDeleteID = palette.id
                                        showDeleteConfirm = true
                                    } label: { Label("删除此主题", systemImage: "trash") }
                                } else {
                                    Button { exportTheme(palette) } label: { Label("导出此主题", systemImage: "square.and.arrow.up") }
                                }
                            }
                        }
                    }
                }
                if !customThemes.isEmpty {
                    HStack(spacing: 4) {
                        Image(systemName: "info.circle").font(.caption2)
                        Text("右键自定义主题可导出或删除")
                            .font(.caption2)
                    }
                    .foregroundStyle(Theme.muted)
                }
            }
        }
        .alert("导入失败", isPresented: $showImportError) {
            Button("好的") {}
        } message: { Text(importError ?? "未知错误") }
        .alert("确认删除", isPresented: $showDeleteConfirm) {
            Button("删除", role: .destructive) { performDelete() }
            Button("取消", role: .cancel) {}
        } message: { Text("删除后将无法恢复，确定删除此自定义主题？") }
    }

    private func importTheme() {
        let panel = NSOpenPanel()
        panel.title = "导入主题"
        panel.allowedContentTypes = [.json]
        panel.allowsMultipleSelection = true
        panel.canChooseDirectories = false
        guard panel.runModal() == .OK else { return }

        var importedCount = 0
        for url in panel.urls {
            do {
                let data = try Data(contentsOf: url)
                var json = try JSONDecoder().decode(ThemeJSON.self, from: data)
                if ThemePalette.builtIn.contains(where: { $0.id == json.id }) {
                    json.id = json.id + "-custom-\(Int(Date().timeIntervalSince1970))"
                }
                try CustomThemeStore.save(json)
                importedCount += 1
            } catch {
                importError = "\(url.lastPathComponent): \(error.localizedDescription)"
                showImportError = true
                return
            }
        }
        customThemes = CustomThemeStore.loadAll()
        if importedCount == 1, let last = customThemes.last {
            model.themeID = last.id
        }
    }

    private func exportCurrent() {
        let palette = ThemePalette.named(model.themeID)
        exportTheme(palette)
    }

    private func exportTheme(_ palette: ThemePalette) {
        let json = ThemeJSON.from(palette)
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        guard let data = try? encoder.encode(json) else { return }

        let panel = NSSavePanel()
        panel.title = "导出主题"
        panel.nameFieldStringValue = "\(palette.id).json"
        panel.allowedContentTypes = [.json]
        guard panel.runModal() == .OK, let url = panel.url else { return }
        try? data.write(to: url, options: .atomic)
    }

    private func performDelete() {
        guard let id = pendingDeleteID else { return }
        CustomThemeStore.delete(id: id)
        customThemes = CustomThemeStore.loadAll()
        if model.themeID == id {
            model.themeID = "cream"
        }
        pendingDeleteID = nil
    }
}

// Miniature app mock-up rendered in a palette's own colors, used as the
// theme switcher preview so users see the skin before applying it.
struct ThemeCard: View {
    let palette: ThemePalette
    let selected: Bool
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            VStack(spacing: 8) {
                ZStack {
                    RoundedRectangle(cornerRadius: min(palette.radius, 9)).fill(palette.canvas)
                    HStack(spacing: 5) {
                        RoundedRectangle(cornerRadius: 2.5)
                            .fill(palette.sidebar)
                            .frame(width: 22)
                            .overlay(alignment: .top) {
                                VStack(spacing: 3) {
                                    Capsule().fill(palette.cyan.opacity(0.85)).frame(width: 14, height: 3)
                                    Capsule().fill(palette.muted.opacity(0.5)).frame(width: 14, height: 3)
                                    Capsule().fill(palette.muted.opacity(0.5)).frame(width: 14, height: 3)
                                }
                                .padding(.top, 7)
                            }
                        VStack(spacing: 4) {
                            RoundedRectangle(cornerRadius: 2.5)
                                .fill(palette.panel)
                                .overlay(
                                    HStack(spacing: 3) {
                                        Circle().fill(palette.cyan).frame(width: 5, height: 5)
                                        Circle().fill(palette.lime).frame(width: 5, height: 5)
                                        Circle().fill(palette.yellow).frame(width: 5, height: 5)
                                        Circle().fill(palette.coral).frame(width: 5, height: 5)
                                    }
                                )
                                .overlay(RoundedRectangle(cornerRadius: 2.5).stroke(palette.border, lineWidth: 0.5))
                            RoundedRectangle(cornerRadius: 2.5)
                                .fill(palette.panel)
                                .overlay(alignment: .bottomLeading) {
                                    HStack(alignment: .bottom, spacing: 2.5) {
                                        ForEach(0..<7, id: \.self) { i in
                                            Capsule()
                                                .fill(palette.cyan.opacity(0.75))
                                                .frame(width: 3, height: [7, 11, 5, 13, 9, 15, 6][i])
                                        }
                                    }
                                    .padding(5)
                                }
                                .overlay(RoundedRectangle(cornerRadius: 2.5).stroke(palette.border, lineWidth: 0.5))
                        }
                    }
                    .padding(6)
                }
                .frame(width: 128, height: 82)
                .overlay(
                    RoundedRectangle(cornerRadius: min(palette.radius, 9))
                        .stroke(selected ? Theme.cyan : Theme.border, lineWidth: selected ? 1.8 : 0.7)
                )
                .overlay(alignment: .topTrailing) {
                    if selected {
                        Image(systemName: "checkmark.circle.fill")
                            .font(.system(size: 13))
                            .foregroundStyle(Theme.cyan)
                            .background(Circle().fill(palette.panel))
                            .offset(x: 5, y: -5)
                    }
                }
                .overlay(alignment: .topLeading) {
                    if !palette.isBuiltIn {
                        Image(systemName: "paintbrush.pointed")
                            .font(.system(size: 9))
                            .foregroundStyle(.white)
                            .padding(3)
                            .background(Circle().fill(Theme.coral.opacity(0.85)))
                            .offset(x: -4, y: -4)
                    }
                }
                Text(palette.name)
                    .font(.system(size: 11, weight: selected ? .semibold : .regular, design: palette.fontDesign))
                    .foregroundStyle(selected ? Color.primary : Theme.muted)
            }
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }
}
