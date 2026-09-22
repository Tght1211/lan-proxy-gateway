import AppKit
import Charts
import SwiftUI
import UniformTypeIdentifiers

// MARK: - Serializable theme for import/export

struct ThemeJSON: Codable {
    var id: String
    var name: String
    var isDark: Bool
    var canvas: [Double]        // [r, g, b]
    var sidebar: [Double]
    var panel: [Double]
    var panelRaised: [Double]
    var border: [Double]
    var cyan: [Double]
    var coral: [Double]
    var lime: [Double]
    var yellow: [Double]
    var muted: [Double]
    var radius: Double
    var radiusSmall: Double
    var borderWidth: Double
    var shadowOpacity: Double
    var shadowRadius: Double
    var fontDesign: String      // "default" | "rounded" | "serif" | "monospaced"
    var canvasGradient: [[Double]]  // [[r,g,b], [r,g,b]] or []

    func toPalette() -> ThemePalette {
        ThemePalette(
            id: id, name: name, isDark: isDark,
            canvas: c(canvas), sidebar: c(sidebar), panel: c(panel), panelRaised: c(panelRaised),
            border: c(border), cyan: c(cyan), coral: c(coral), lime: c(lime), yellow: c(yellow),
            muted: c(muted),
            radius: CGFloat(radius), radiusSmall: CGFloat(radiusSmall),
            borderWidth: CGFloat(borderWidth), shadowOpacity: shadowOpacity, shadowRadius: CGFloat(shadowRadius),
            fontDesign: fd(fontDesign),
            canvasGradient: canvasGradient.map { c($0) },
            isBuiltIn: false
        )
    }

    private func c(_ rgb: [Double]) -> Color {
        Color(red: rgb.count > 0 ? rgb[0] : 0, green: rgb.count > 1 ? rgb[1] : 0, blue: rgb.count > 2 ? rgb[2] : 0)
    }
    private func fd(_ s: String) -> Font.Design {
        switch s {
        case "rounded": return .rounded
        case "serif": return .serif
        case "monospaced": return .monospaced
        default: return .default
        }
    }

    static func from(_ p: ThemePalette) -> ThemeJSON {
        ThemeJSON(
            id: p.id, name: p.name, isDark: p.isDark,
            canvas: rgb(p.canvas), sidebar: rgb(p.sidebar), panel: rgb(p.panel), panelRaised: rgb(p.panelRaised),
            border: rgb(p.border), cyan: rgb(p.cyan), coral: rgb(p.coral), lime: rgb(p.lime), yellow: rgb(p.yellow),
            muted: rgb(p.muted),
            radius: Double(p.radius), radiusSmall: Double(p.radiusSmall),
            borderWidth: Double(p.borderWidth), shadowOpacity: p.shadowOpacity, shadowRadius: Double(p.shadowRadius),
            fontDesign: fdStr(p.fontDesign),
            canvasGradient: p.canvasGradient.map { rgb($0) }
        )
    }

    private static func rgb(_ color: Color) -> [Double] {
        let c = NSColor(color).usingColorSpace(.sRGB) ?? NSColor(color)
        return [Double(c.redComponent), Double(c.greenComponent), Double(c.blueComponent)]
            .map { (($0 * 1000).rounded()) / 1000 }
    }
    private static func fdStr(_ d: Font.Design) -> String {
        switch d {
        case .rounded: return "rounded"
        case .serif: return "serif"
        case .monospaced: return "monospaced"
        default: return "default"
        }
    }
}

// MARK: - Custom theme storage

enum CustomThemeStore {
    static let dir: URL = {
        let d = FileManager.default.homeDirectoryForCurrentUser
            .appendingPathComponent(".config/lan-proxy-gateway/themes", isDirectory: true)
        try? FileManager.default.createDirectory(at: d, withIntermediateDirectories: true)
        return d
    }()

    static func loadAll() -> [ThemePalette] {
        guard let files = try? FileManager.default.contentsOfDirectory(at: dir, includingPropertiesForKeys: nil)
            .filter({ $0.pathExtension == "json" }) else { return [] }
        return files.compactMap { url in
            guard let data = try? Data(contentsOf: url),
                  let json = try? JSONDecoder().decode(ThemeJSON.self, from: data) else { return nil }
            return json.toPalette()
        }
    }

    // The id comes from untrusted imported JSON and is used as a filename:
    // restrict it to a safe charset so "../x" cannot escape the themes dir.
    static func fileURL(forID id: String) -> URL? {
        let allowed = CharacterSet.alphanumerics.union(CharacterSet(charactersIn: "-_"))
        guard !id.isEmpty, id.count <= 64,
              id.unicodeScalars.allSatisfy({ allowed.contains($0) }) else { return nil }
        return dir.appendingPathComponent("\(id).json")
    }

    static func save(_ theme: ThemeJSON) throws {
        guard let url = fileURL(forID: theme.id) else {
            throw NSError(domain: "CustomThemeStore", code: 1, userInfo: [
                NSLocalizedDescriptionKey: "主题 id 不合法（仅允许字母、数字、- 和 _，最长 64 字符）: \(theme.id)",
            ])
        }
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        let data = try encoder.encode(theme)
        try data.write(to: url, options: .atomic)
    }

    static func delete(id: String) {
        guard let url = fileURL(forID: id) else { return }
        try? FileManager.default.removeItem(at: url)
    }
}

// MARK: - ThemePalette

struct ThemePalette: Identifiable {
    let id: String
    let name: String
    let isDark: Bool
    let canvas: Color
    let sidebar: Color
    let panel: Color
    let panelRaised: Color
    let border: Color
    let cyan: Color
    let coral: Color
    let lime: Color
    let yellow: Color
    let muted: Color
    let radius: CGFloat
    let radiusSmall: CGFloat
    let borderWidth: CGFloat
    let shadowOpacity: Double
    let shadowRadius: CGFloat
    let fontDesign: Font.Design
    let canvasGradient: [Color]
    var isBuiltIn: Bool = true

    static let light = ThemePalette(
        id: "light",
        name: "经典浅色",
        isDark: false,
        canvas: Color(red: 0.955, green: 0.961, blue: 0.969),
        sidebar: Color(red: 0.925, green: 0.937, blue: 0.945),
        panel: Color.white,
        panelRaised: Color(red: 0.969, green: 0.975, blue: 0.979),
        border: Color(red: 0.835, green: 0.855, blue: 0.875),
        cyan: Color(red: 0.08, green: 0.42, blue: 0.36),
        coral: Color(red: 0.72, green: 0.20, blue: 0.18),
        lime: Color(red: 0.20, green: 0.52, blue: 0.28),
        yellow: Color(red: 0.78, green: 0.47, blue: 0.08),
        muted: Color(nsColor: .secondaryLabelColor),
        radius: 7,
        radiusSmall: 6,
        borderWidth: 0.7,
        shadowOpacity: 0.035,
        shadowRadius: 7,
        fontDesign: .default,
        canvasGradient: []
    )

    static let graphite = ThemePalette(
        id: "graphite",
        name: "石墨深色",
        isDark: true,
        canvas: Color(red: 0.082, green: 0.09, blue: 0.106),
        sidebar: Color(red: 0.104, green: 0.114, blue: 0.133),
        panel: Color(red: 0.125, green: 0.137, blue: 0.157),
        panelRaised: Color(red: 0.16, green: 0.175, blue: 0.20),
        border: Color(red: 0.255, green: 0.28, blue: 0.318),
        cyan: Color(red: 0.30, green: 0.76, blue: 0.66),
        coral: Color(red: 0.94, green: 0.45, blue: 0.40),
        lime: Color(red: 0.55, green: 0.79, blue: 0.40),
        yellow: Color(red: 0.94, green: 0.70, blue: 0.32),
        muted: Color(red: 0.60, green: 0.64, blue: 0.70),
        radius: 3,
        radiusSmall: 2,
        borderWidth: 1.0,
        shadowOpacity: 0.5,
        shadowRadius: 12,
        fontDesign: .default,
        canvasGradient: [
            Color(red: 0.098, green: 0.106, blue: 0.128),
            Color(red: 0.066, green: 0.072, blue: 0.086),
        ]
    )

    static let ocean = ThemePalette(
        id: "ocean",
        name: "海雾蓝",
        isDark: false,
        canvas: Color(red: 0.928, green: 0.947, blue: 0.965),
        sidebar: Color(red: 0.885, green: 0.913, blue: 0.941),
        panel: Color.white,
        panelRaised: Color(red: 0.945, green: 0.960, blue: 0.975),
        border: Color(red: 0.80, green: 0.85, blue: 0.895),
        cyan: Color(red: 0.10, green: 0.36, blue: 0.65),
        coral: Color(red: 0.78, green: 0.23, blue: 0.22),
        lime: Color(red: 0.12, green: 0.50, blue: 0.44),
        yellow: Color(red: 0.79, green: 0.50, blue: 0.10),
        muted: Color(nsColor: .secondaryLabelColor),
        radius: 13,
        radiusSmall: 9,
        borderWidth: 0.5,
        shadowOpacity: 0.08,
        shadowRadius: 16,
        fontDesign: .rounded,
        canvasGradient: [
            Color(red: 0.895, green: 0.925, blue: 0.955),
            Color(red: 0.945, green: 0.958, blue: 0.968),
        ]
    )

    static let cream = ThemePalette(
        id: "cream",
        name: "暖沙米",
        isDark: false,
        canvas: Color(red: 0.960, green: 0.943, blue: 0.912),
        sidebar: Color(red: 0.928, green: 0.903, blue: 0.862),
        panel: Color(red: 0.995, green: 0.986, blue: 0.968),
        panelRaised: Color(red: 0.963, green: 0.948, blue: 0.922),
        border: Color(red: 0.78, green: 0.73, blue: 0.65),
        cyan: Color(red: 0.62, green: 0.31, blue: 0.14),
        coral: Color(red: 0.74, green: 0.22, blue: 0.18),
        lime: Color(red: 0.35, green: 0.51, blue: 0.25),
        yellow: Color(red: 0.71, green: 0.49, blue: 0.10),
        muted: Color(nsColor: .secondaryLabelColor),
        radius: 8,
        radiusSmall: 6,
        borderWidth: 1.1,
        shadowOpacity: 0,
        shadowRadius: 0,
        fontDesign: .serif,
        canvasGradient: []
    )

    static let builtIn: [ThemePalette] = [.light, .graphite, .ocean, .cream]

    static var all: [ThemePalette] {
        builtIn + CustomThemeStore.loadAll()
    }

    static func named(_ id: String) -> ThemePalette {
        all.first { $0.id == id } ?? .cream
    }
}

enum Theme {
    static var palette = ThemePalette.cream
    static var canvas: Color { palette.canvas }
    static var sidebar: Color { palette.sidebar }
    static var panel: Color { palette.panel }
    static var panelRaised: Color { palette.panelRaised }
    static var border: Color { palette.border }
    static var cyan: Color { palette.cyan }
    static var coral: Color { palette.coral }
    static var lime: Color { palette.lime }
    static var yellow: Color { palette.yellow }
    static var muted: Color { palette.muted }
    static var radius: CGFloat { palette.radius }
    static var radiusSmall: CGFloat { palette.radiusSmall }
    static var borderWidth: CGFloat { palette.borderWidth }
    static var shadowOpacity: Double { palette.shadowOpacity }
    static var shadowRadius: CGFloat { palette.shadowRadius }

    // Page background: flat color or the skin's vertical gradient.
    static var canvasBackground: LinearGradient {
        let colors = palette.canvasGradient.isEmpty ? [palette.canvas, palette.canvas] : palette.canvasGradient
        return LinearGradient(colors: colors, startPoint: .top, endPoint: .bottom)
    }
}

struct ContentView: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        // Swap the palette before subviews evaluate; .id forces a full rebuild
        // so every cached view picks up the new colors.
        Theme.palette = ThemePalette.named(model.themeID)
        return mainView
            .id(model.themeID)
            .fontDesign(Theme.palette.fontDesign)
            .preferredColorScheme(Theme.palette.isDark ? .dark : .light)
    }

    private var mainView: some View {
        NavigationSplitView {
            sidebar
        } detail: {
            VStack(spacing: 0) {
                TopBar()
                Divider().overlay(Theme.border)
                if model.coreUpgradeRecommended {
                    CoreCompatibilityBar()
                }
                detail
            }
            .background(Theme.canvasBackground)
        }
        .tint(Theme.cyan)
        .alert("操作失败", isPresented: Binding(
            get: { model.errorMessage != nil },
            set: { if !$0 { model.errorMessage = nil } }
        )) {
            Button("关闭", role: .cancel) { model.errorMessage = nil }
        } message: {
            Text(model.errorMessage ?? "未知错误")
        }
        .overlay(alignment: .bottom) {
            if let notice = model.notice {
                NoticeBar(text: notice).padding(.bottom, 18)
            }
        }
        .animation(.easeOut(duration: 0.2), value: model.notice)
    }

    private var sidebar: some View {
        VStack(spacing: 0) {
            HStack(spacing: 11) {
                ZStack {
                    RoundedRectangle(cornerRadius: Theme.radius).fill(Theme.cyan.opacity(0.16))
                    Image(systemName: "network").foregroundStyle(Theme.cyan).font(.system(size: 17, weight: .semibold))
                }
                .frame(width: 32, height: 32)
                VStack(alignment: .leading, spacing: 2) {
                    Text("旁路由").font(.system(size: 14, weight: .semibold))
                    Text("LAN Proxy Gateway").font(.system(size: 10)).foregroundStyle(Theme.muted)
                }
                Spacer()
            }
            .padding(.horizontal, 16)
            .padding(.top, 26)
            .frame(height: 92)

            List(AppSection.allCases, selection: $model.selectedSection) { section in
                Label(section.rawValue, systemImage: section.systemImage)
                    .font(.system(size: 13, weight: .medium))
                    .symbolRenderingMode(.hierarchical)
                    .tag(section)
                    .frame(minHeight: 27)
            }
            .scrollContentBackground(.hidden)
            .scrollIndicators(.hidden)
            .listStyle(.sidebar)

            VStack(alignment: .leading, spacing: 9) {
                HStack(spacing: 8) {
                    Circle().fill(model.isRunning ? Theme.lime : Theme.muted).frame(width: 7, height: 7)
                    Text(model.isRunning ? "服务运行中" : "服务已停止")
                        .font(.system(size: 11, weight: .semibold))
                        .foregroundStyle(model.isRunning ? Theme.lime : Theme.muted)
                }
                Text(model.status?.gateway.localIP.nonEmpty ?? "等待网络检测")
                    .font(.system(size: 12, design: .monospaced)).foregroundStyle(Theme.muted)
            }
            .padding(16)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(Theme.panel.opacity(0.72))
        }
        .background(Theme.sidebar)
        .navigationSplitViewColumnWidth(min: 172, ideal: 184, max: 200)
    }

    @ViewBuilder private var detail: some View {
        Group {
            switch model.selectedSection ?? .overview {
            case .overview: OverviewView()
            case .devices: DevicesView()
            case .connections: ConnectionsView()
            case .settings: SettingsView()
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .top)
    }
}

struct CoreCompatibilityBar: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        HStack(spacing: 10) {
            Image(systemName: "arrow.triangle.2.circlepath.circle.fill").foregroundStyle(Theme.yellow)
            Text("检测到较早版本的核心服务，部分实时数据不可用。")
                .font(.system(size: 12, weight: .medium))
            Spacer()
            Button("使用当前核心重启") { model.restart() }
                .buttonStyle(ActionButtonStyle(tint: Theme.yellow))
                .disabled(model.isBusy)
        }
        .padding(.horizontal, 20)
        .frame(height: 44)
        .background(Theme.yellow.opacity(0.09))
        .overlay(alignment: .bottom) { Divider().overlay(Theme.border) }
    }
}

struct TopBar: View {
    @EnvironmentObject private var model: AppModel

    var body: some View {
        HStack(spacing: 14) {
            VStack(alignment: .leading, spacing: 2) {
                Text(model.selectedSection?.rawValue ?? "网络总览").font(.system(size: 18, weight: .semibold))
                Text(model.isRunning ? "网关服务正常 · \(model.status?.gateway.localIP.nonEmpty ?? "正在检测网络")" : "网关服务未运行")
                    .font(.system(size: 11)).foregroundStyle(Theme.muted)
            }
            Spacer()
            if model.isBusy { ProgressView().controlSize(.small) }
            Button { Task { await model.refresh() } } label: { Image(systemName: "arrow.clockwise") }
                .buttonStyle(IconButtonStyle()).help("刷新")
            if model.isRunning {
                Button { model.restart() } label: { Label("重启", systemImage: "arrow.triangle.2.circlepath") }
                    .buttonStyle(.bordered).help("重启核心服务")
                Button { model.stop() } label: { Label("停止核心", systemImage: "stop.fill") }
                    .buttonStyle(ActionButtonStyle(tint: Theme.coral))
            } else {
                Button { model.initializeAndStart() } label: {
                    Label(model.isConfigured ? "启动核心" : "初始化核心", systemImage: "play.fill")
                }
                .buttonStyle(ActionButtonStyle(tint: Theme.lime))
            }
        }
        .padding(.horizontal, 24)
        .frame(height: 58)
        .background(Theme.canvasBackground)
        .disabled(model.isBusy)
    }
}

// Layout rule: every content page either uses ScrollPage (scrollable, panels
// stack freely) or is a full-height page where ONLY flexible views (Table,
// TextEditor) absorb remaining space. Fixed-height stacks that can outgrow
// the window overflow-center and shove the whole split view off screen.
struct ScrollPage<Content: View>: View {
    @ViewBuilder let content: Content
    var body: some View {
        ScrollView(.vertical, showsIndicators: false) {
            VStack(spacing: 16) { content }
                .padding(20)
                .frame(maxWidth: .infinity)
        }
    }
}
