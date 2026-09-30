import AppKit
import SwiftUI

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
        Group {
            classicView
        }
        .tint(Theme.cyan)
        .foregroundStyle(Theme.text)
        .buttonStyle(StudioButtonStyle())
        .alert("操作失败", isPresented: Binding(
            get: { model.errorMessage != nil },
            set: { if !$0 { model.errorMessage = nil } }
        )) {
            Button("关闭", role: .cancel) { model.errorMessage = nil }
        } message: { Text(model.errorMessage ?? "未知错误") }
        .overlay(alignment: .bottom) {
            if let notice = model.notice { NoticeBar(text: notice).padding(.bottom, 18) }
        }
    }

    private var classicView: some View {
        NavigationSplitView {
            sidebar
        } detail: {
            VStack(spacing: 0) {
                TopBar()
                if model.coreUpgradeRecommended {
                    CoreCompatibilityBar()
                }
                detail
            }
            .background(Theme.canvasBackground)
        }
    }

    private var sidebar: some View {
        VStack(alignment: .leading, spacing: 0) {
            Text("LAN Proxy\nGateway").font(.system(size: 19, weight: .bold))
                .tracking(-0.4).padding(.horizontal, 8).padding(.top, 30).padding(.bottom, 28)
            VStack(spacing: 5) {
                ForEach(AppSection.allCases) { section in
                    Button { model.selectedSection = section } label: {
                        HStack(spacing: 10) {
                            StudioIcon(section.systemImage).frame(width: 18, height: 18)
                            Text(section.rawValue).font(.system(size: 13, weight: model.selectedSection == section ? .semibold : .regular))
                            Spacer(minLength: 0)
                        }.foregroundStyle(model.selectedSection == section ? Theme.cyan : Theme.muted)
                            .padding(.horizontal, 11).padding(.vertical, 12)
                            .background(model.selectedSection == section ? Theme.soft : .clear)
                            .clipShape(RoundedRectangle(cornerRadius: 8)).contentShape(Rectangle())
                    }.buttonStyle(.plain).accessibilityAddTraits(model.selectedSection == section ? [.isSelected] : [])
                }
            }
            Spacer()
            VStack(alignment: .leading, spacing: 8) {
                HStack(spacing: 7) {
                    Circle().fill(model.isRunning ? Theme.lime : Theme.muted).frame(width: 6, height: 6)
                    Text(model.isRunning ? "网关运行中" : "网关已停止").font(.system(size: 11))
                }
                Text(model.status?.gateway.localIP.nonEmpty ?? "等待网络检测")
                    .font(.system(size: 11, design: .monospaced)).foregroundStyle(Theme.muted)
            }.padding(8).padding(.bottom, 16)
        }.padding(.horizontal, 12).background(Theme.sidebar)
            .navigationSplitViewColumnWidth(min: 170, ideal: 180, max: 210)
    }

    @ViewBuilder private var detail: some View {
        Group {
            switch model.selectedSection ?? .overview {
            case .overview: NetworkOverviewView()
            case .exits: NetworkExitsView()
            case .rules: NetworkRulesView()
            case .devices: NetworkDevicesView()
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
    @State private var proxySheet = false
    @State private var directSheet = false
    var body: some View {
        HStack(spacing: 12) {
            Text(model.selectedSection?.rawValue ?? "网络总览")
                .font(.system(size: 25, weight: .semibold)).tracking(-0.6)
            Spacer()
            if model.isBusy { ProgressView().controlSize(.small) }
            HStack(spacing: 14) {
                Text("出口").foregroundStyle(Theme.muted)
                if model.status?.proxy?.isEmpty == false {
                    Button { proxySheet = true } label: {
                        HStack(spacing: 6) {
                            Circle().fill(model.stats?.health.healthy == true ? Theme.lime : Theme.muted).frame(width: 6,height: 6)
                            Text("代理")
                        }
                    }.buttonStyle(.plain)
                }
                Button { directSheet = true } label: {
                    HStack(spacing: 6) { Circle().fill(model.isRunning ? Theme.lime : Theme.muted).frame(width: 6,height: 6); Text("直连") }
                }.buttonStyle(.plain)
            }.font(.system(size: 12)).padding(.horizontal, 12).padding(.vertical, 11)
                .background(Theme.panel).clipShape(RoundedRectangle(cornerRadius: 9))
            Menu {
                Button("刷新状态") { Task { await model.refresh() } }
                if model.isRunning {
                    Button("重启核心") { model.restart() }
                    Button("停止核心", role: .destructive) { model.stop() }
                } else { Button("启动核心") { model.initializeAndStart() } }
            } label: { Image(systemName: "ellipsis").font(.system(size: 17)) }
                .menuStyle(.borderlessButton).menuIndicator(.hidden).frame(width: 22).help("核心操作")
        }.padding(.horizontal, 22).padding(.top, 16).padding(.bottom, 6)
            .background(Theme.canvas)
            .sheet(isPresented: $proxySheet) { NetworkSheetContent(destination: .proxy) }
            .sheet(isPresented: $directSheet) { NetworkSheetContent(destination: .direct) }
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
            VStack(alignment: .leading, spacing: 22) { content }
                .padding(22)
                .frame(maxWidth: .infinity)
        }
    }
}
