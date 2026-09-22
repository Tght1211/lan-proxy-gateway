import AppKit
import SwiftUI

@main
struct LANProxyGatewayApp: App {
    @StateObject private var model = AppModel()

    init() {
        if let url = Bundle.main.url(forResource: "AppIcon", withExtension: "icns")
            ?? Bundle.module.url(forResource: "AppIcon", withExtension: "icns"),
           let icon = NSImage(contentsOf: url) {
            NSApplication.shared.applicationIconImage = icon
        }
    }

    var body: some Scene {
        WindowGroup("LAN Proxy Gateway", id: "main") {
            ContentView()
                .environmentObject(model)
                .frame(minWidth: 1080, minHeight: 700)
                .task { StatusBarController.shared.install(model: model) }
        }
        .windowStyle(.hiddenTitleBar)
        .defaultSize(width: 1280, height: 820)
    }
}
