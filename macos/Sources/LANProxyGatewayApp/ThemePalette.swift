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
        id: "light", name: "经典浅色", isDark: false,
        canvas: Color(hex: 0xf0f2f5), sidebar: Color.white,
        panel: Color.white, panelRaised: Color(hex: 0xf9fafc),
        border: Color(hex: 0xdce3e9), cyan: Color(hex: 0x087d72),
        coral: Color(hex: 0xbc495b), lime: Color(hex: 0x388c49),
        yellow: Color(hex: 0xa76b15), muted: Color(hex: 0x667483),
        radius: 14, radiusSmall: 8, borderWidth: 0,
        shadowOpacity: 0, shadowRadius: 0, fontDesign: .default, canvasGradient: []
    )

    static let graphite = ThemePalette(
        id: "graphite", name: "石墨深色", isDark: true,
        canvas: Color(hex: 0x15191e), sidebar: Color(hex: 0x20262d),
        panel: Color(hex: 0x20262d), panelRaised: Color(hex: 0x262e37),
        border: Color(hex: 0x38444f), cyan: Color(hex: 0x55ceba),
        coral: Color(hex: 0xef7e91), lime: Color(hex: 0x8cc66b),
        yellow: Color(hex: 0xefb952), muted: Color(hex: 0x9aa9b8),
        radius: 14, radiusSmall: 8, borderWidth: 0,
        shadowOpacity: 0, shadowRadius: 0, fontDesign: .default, canvasGradient: []
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
    static var text: Color { palette.isDark ? Color(hex: 0xecf1f6) : Color(hex: 0x202a36) }
    static var soft: Color { palette.isDark ? Color(hex: 0x263d3c) : Color(hex: 0xe3f3ee) }
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


extension Color {
    init(hex: UInt32) {
        self.init(red: Double((hex >> 16) & 255) / 255, green: Double((hex >> 8) & 255) / 255, blue: Double(hex & 255) / 255)
    }
}
