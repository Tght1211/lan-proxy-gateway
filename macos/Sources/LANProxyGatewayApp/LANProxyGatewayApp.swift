import SwiftUI

@main
struct LANProxyGatewayApp: App {
    @StateObject private var model = AppModel()

    var body: some Scene {
        WindowGroup("LAN Proxy Gateway", id: "main") {
            ContentView()
                .environmentObject(model)
                .frame(minWidth: 1080, minHeight: 700)
        }
        .windowStyle(.hiddenTitleBar)
        .defaultSize(width: 1280, height: 820)

        MenuBarExtra {
            Button("打开控制台") {
                NSApp.activate(ignoringOtherApps: true)
                if let window = NSApp.windows.first { window.makeKeyAndOrderFront(nil) }
            }
            Divider()
            if model.isRunning {
                Button("重启核心服务") { model.restart() }
                Button("停止核心服务") { model.stop() }
            } else {
                Button(model.isConfigured ? "启动核心服务" : "初始化并启动") {
                    model.initializeAndStart()
                }
            }
            Divider()
            Button("退出") { NSApp.terminate(nil) }
        } label: {
            Image(systemName: model.isRunning ? "network.badge.shield.half.filled" : "network.slash")
        }
        .menuBarExtraStyle(.menu)
    }
}
