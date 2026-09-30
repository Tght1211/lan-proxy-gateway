import SwiftUI

struct NetworkRulesView: View {
    @EnvironmentObject private var model: AppModel
    @State private var tab = "rules"
    @State private var initialRules: [RoutingRule]?
    var body: some View {
        VStack(alignment:.leading,spacing:18) {
            StudioTabs(title: "规则配置分类", selection: $tab, items: [
                ("rules", "代理规则", "list.bullet.rectangle"), ("learning", "自学习", "sparkles"), ("exits", "网络出口", "route")
            ]).padding(.horizontal,22).padding(.top,16)
            ZStack {
                // Keep the editor alive when switching tabs, preserving unsaved text/list drafts.
                if let initialRules { RoutingRulesEditor(rules:initialRules,embedded:true)
                    .opacity(tab == "rules" ? 1:0).allowsHitTesting(tab == "rules").accessibilityHidden(tab != "rules").clipShape(RoundedRectangle(cornerRadius:12)).overlay(RoundedRectangle(cornerRadius:12).stroke(Theme.border,lineWidth:0.8)).padding(.horizontal,22).padding(.bottom,22)
                } else { ProgressView("读取规则…") }
                if tab == "learning" { ScrollPage { NetworkLearningView() }.background(Theme.canvas) }
                if tab == "exits" { NetworkExitsView().background(Theme.canvas) }
            }
        }.onReceive(model.$status) { status in
            if initialRules == nil, let status { initialRules = status.routing ?? [] }
        }
    }
}

struct NetworkLearningView: View {
    @EnvironmentObject private var model: AppModel
    @State private var editRules = false
    var body: some View {
        VStack(alignment:.leading,spacing:20) {
            LearningSettingsView()
            Panel {
                VStack(alignment:.leading,spacing:12) {
                    Text("学习记录").font(.headline)
                    FallbackLearningPopover(onEditRules: { editRules = true }, embedded:true)
                }.frame(maxWidth:.infinity,alignment:.leading)
            }
        }.sheet(isPresented: $editRules) { RoutingRulesEditor(rules: model.status?.routing ?? []) }
    }
}
