import AppKit
import Combine
import SwiftUI

/// 菜单栏图标（与 App 品牌一致的四节点拓扑，template 适配深浅色菜单栏）。
enum AppBrandIcon {
    private static let menuBarSize = NSSize(width: 18, height: 18)

    static func menuBarImage(isRunning: Bool) -> NSImage {
        let image = drawTopologyTemplate(size: menuBarSize, alpha: isRunning ? 1 : 0.45)
        image.isTemplate = true
        return image
    }

    /// 与 generate_icon.swift 同源几何，缩放到菜单栏尺寸。template 图以 alpha 表达明暗。
    private static func drawTopologyTemplate(size: NSSize, alpha: CGFloat) -> NSImage {
        let image = NSImage(size: size)
        image.lockFocus()
        guard let ctx = NSGraphicsContext.current?.cgContext else {
            image.unlockFocus()
            return image
        }
        let color = NSColor.black.withAlphaComponent(alpha).cgColor
        ctx.setFillColor(color)
        ctx.setStrokeColor(color)
        ctx.setLineCap(.round)

        let scale = size.width / 1024
        func point(_ x: CGFloat, _ y: CGFloat) -> CGPoint {
            CGPoint(x: x * scale, y: y * scale)
        }

        let center = point(512, 512)
        let nodes = [point(300, 710), point(724, 710), point(300, 314), point(724, 314)]
        let lineWidth = max(1.1, 38 * scale)
        let nodeR = max(1.8, 66 * scale)

        for node in nodes {
            ctx.setLineWidth(lineWidth)
            ctx.move(to: center)
            ctx.addLine(to: node)
            ctx.strokePath()
            ctx.fillEllipse(in: CGRect(x: node.x - nodeR, y: node.y - nodeR, width: nodeR * 2, height: nodeR * 2))
        }

        let gw = CGRect(x: 374 * scale, y: 396 * scale, width: 276 * scale, height: 232 * scale)
        let path = CGPath(roundedRect: gw, cornerWidth: 58 * scale, cornerHeight: 58 * scale, transform: nil)
        ctx.addPath(path)
        ctx.fillPath()

        let dotR = max(0.6, 18 * scale)
        for x in [438.0, 512.0, 586.0] {
            let c = point(x, 498)
            ctx.fillEllipse(in: CGRect(x: c.x - dotR, y: c.y - dotR, width: dotR * 2, height: dotR * 2))
        }

        image.unlockFocus()
        return image
    }
}

/// 用 AppKit 状态栏替代 MenuBarExtra，避免系统把彩色图标替换成默认「地球」。
@MainActor
final class StatusBarController: ObservableObject {
    static let shared = StatusBarController()

    private var statusItem: NSStatusItem?
    private var model: AppModel?
    private var cancellable: AnyCancellable?

    func install(model: AppModel) {
        guard statusItem == nil else {
            self.model = model
            refreshIcon()
            return
        }
        self.model = model
        let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)
        item.button?.image = AppBrandIcon.menuBarImage(isRunning: model.isRunning)
        item.button?.imagePosition = .imageOnly
        item.button?.toolTip = "LAN Proxy Gateway"
        item.menu = buildMenu()
        statusItem = item

        cancellable = model.$status
            .receive(on: RunLoop.main)
            .sink { [weak self] (_: GatewayStatus?) in self?.refreshIcon() }
    }

    private func refreshIcon() {
        guard let model else { return }
        statusItem?.button?.image = AppBrandIcon.menuBarImage(isRunning: model.isRunning)
        statusItem?.menu = buildMenu()
    }

    private func buildMenu() -> NSMenu {
        let menu = NSMenu()
        menu.addItem(withTitle: "打开控制台", action: #selector(openConsole), keyEquivalent: "")
        menu.addItem(.separator())

        guard let model else { return menu }
        if model.isRunning {
            menu.addItem(withTitle: "重启核心服务", action: #selector(restartCore), keyEquivalent: "")
            menu.addItem(withTitle: "停止核心服务", action: #selector(stopCore), keyEquivalent: "")
        } else {
            let title = model.isConfigured ? "启动核心服务" : "初始化并启动"
            menu.addItem(withTitle: title, action: #selector(startCore), keyEquivalent: "")
        }
        menu.addItem(.separator())
        menu.addItem(withTitle: "退出", action: #selector(quit), keyEquivalent: "q")

        for item in menu.items {
            item.target = self
        }
        return menu
    }

    @objc private func openConsole() {
        NSApp.activate(ignoringOtherApps: true)
        NSApp.windows.first { $0.canBecomeMain }?.makeKeyAndOrderFront(nil)
    }

    @objc private func restartCore() { model?.restart() }
    @objc private func stopCore() { model?.stop() }
    @objc private func startCore() { model?.initializeAndStart() }
    @objc private func quit() { NSApp.terminate(nil) }
}
