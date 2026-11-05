import AppKit
import SwiftUI

struct RoutingReview: Identifiable {
    let id = UUID()
    let localChanges: [RoutingRuleChange]
    let externalChanges: [RoutingRuleChange]
    let merge: RoutingRuleMerge?
}

struct RoutingChangesSheet: View {
    let review: RoutingReview
    let onResolve: ([RoutingRule], [RoutingRule]) -> Void
    @Environment(\.dismiss) private var dismiss
    @State private var section = "local"
    @State private var choices: [UUID: RoutingRuleMerge.Choice] = [:]
    @State private var orderChoice: RoutingRuleMerge.Choice?

    private var changes: [RoutingRuleChange] { section == "local" ? review.localChanges : review.externalChanges }
    private var resolved: [RoutingRule]? { review.merge?.resolved(choices: choices, order: orderChoice) }

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            HStack {
                VStack(alignment: .leading, spacing: 5) {
                    Text("规则变更对比").font(.title2.bold())
                    Text("仅展示发生变化的规则；分组、动作、来源和全局优先级都会参与对比。")
                        .font(.caption).foregroundStyle(Theme.muted)
                }
                Spacer()
                Button("关闭") { dismiss() }.buttonStyle(StudioButtonStyle())
            }
            Picker("对比范围", selection: $section) {
                Text("我的修改 \(review.localChanges.count)").tag("local")
                Text("外部更新 \(review.externalChanges.count)").tag("external")
                if let merge = review.merge, merge.needsResolution {
                    Text("需要处理 \(merge.conflicts.count + (merge.orderConflict ? 1 : 0))").tag("conflicts")
                }
            }.pickerStyle(.segmented)
            if section == "conflicts", let merge = review.merge {
                Text("仅选择冲突规则的版本，其他修改会自动保留。合并后回到编辑器预览，点击“应用规则”才会保存。")
                    .font(.caption).foregroundStyle(Theme.yellow)
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: 12) {
                        if merge.orderConflict {
                            VStack(alignment: .leading, spacing: 8) {
                                Text("双方都调整了规则优先级").font(.headline)
                                Text("内容修改与新增规则仍会保留；请选择全局规则顺序。具体位置可在上方两个对比页查看。")
                                    .font(.caption).foregroundStyle(Theme.muted)
                                HStack {
                                    choiceButton("采用我的顺序", selected: orderChoice == .local) { orderChoice = .local }
                                    choiceButton("采用最新顺序", selected: orderChoice == .latest) { orderChoice = .latest }
                                }
                            }.padding(12).background(Theme.panelRaised).clipShape(RoundedRectangle(cornerRadius: 8))
                        }
                        ForEach(merge.conflicts) { conflict in
                            VStack(alignment: .leading, spacing: 10) {
                                Text((conflict.local ?? conflict.latest ?? conflict.baseline)?.value ?? "规则冲突").font(.headline)
                                ruleVersion("编辑前", rule: conflict.baseline, color: Theme.muted)
                                HStack(alignment: .top, spacing: 12) {
                                    ruleVersion("我的草稿", rule: conflict.local, color: Theme.cyan)
                                    ruleVersion("外部最新", rule: conflict.latest, color: Theme.yellow)
                                }
                                HStack {
                                    choiceButton("保留我的修改", selected: choices[conflict.id] == .local) { choices[conflict.id] = .local }
                                    choiceButton("采用外部更新", selected: choices[conflict.id] == .latest) { choices[conflict.id] = .latest }
                                }
                            }.padding(12).background(Theme.panelRaised).clipShape(RoundedRectangle(cornerRadius: 8))
                        }
                    }
                }
            } else {
                HStack(spacing: 14) {
                    ForEach(["新增", "删除", "修改", "调整顺序"], id: \.self) { kind in
                        Text("\(kind) \(changes.filter { $0.kind == kind }.count)")
                            .font(.caption.monospacedDigit()).foregroundStyle(tint(kind))
                    }
                }
                if changes.isEmpty {
                    Text(section == "local" ? "没有未应用的规则修改。" : "没有新的外部规则更新。")
                        .foregroundStyle(Theme.muted).frame(maxWidth: .infinity, maxHeight: .infinity)
                } else {
                    ScrollView {
                        LazyVStack(alignment: .leading, spacing: 10) {
                            ForEach(changes) { change in
                                VStack(alignment: .leading, spacing: 8) {
                                    HStack {
                                        Text(change.kind).font(.caption.bold()).foregroundStyle(tint(change.kind))
                                        Text(change.title).font(.headline).textSelection(.enabled)
                                        Spacer()
                                        Text(change.details).font(.caption).foregroundStyle(Theme.muted)
                                    }
                                    HStack(alignment: .top, spacing: 12) {
                                        ruleVersion("变更前\(change.oldPosition.map { " · #\($0)" } ?? "")", rule: change.before, color: Theme.coral)
                                        ruleVersion("变更后\(change.newPosition.map { " · #\($0)" } ?? "")", rule: change.after, color: Theme.lime)
                                    }
                                }.padding(12).background(Theme.panelRaised).clipShape(RoundedRectangle(cornerRadius: 8))
                            }
                        }
                    }
                }
            }
            Divider()
            HStack {
                Button("复制差异") { copyDiff() }.buttonStyle(StudioButtonStyle())
                Spacer()
                if let merge = review.merge {
                    Button("合并到草稿") {
                        guard let resolved else { return }
                        onResolve(resolved, merge.latest)
                        dismiss()
                    }.buttonStyle(ActionButtonStyle(tint: Theme.cyan)).disabled(resolved == nil)
                } else {
                    Text("差异查看不会修改已生效的规则。").font(.caption).foregroundStyle(Theme.muted)
                }
            }
        }
        .padding(22).frame(width: 900, height: 650).background(Theme.canvasBackground)
        .onAppear { if review.merge?.needsResolution == true { section = "conflicts" } }
    }

    private func ruleVersion(_ title: String, rule: RoutingRule?, color: Color) -> some View {
        VStack(alignment: .leading, spacing: 5) {
            Text(title).font(.caption.bold()).foregroundStyle(color)
            if let rule {
                Text("\(rule.type) · \(rule.value)").font(.system(size: 12, design: .monospaced)).textSelection(.enabled)
                Text("\(rule.action == "proxy" ? "代理" : rule.action == "direct" ? "直连" : "拒绝") · \(rule.group.isEmpty ? "未分组" : rule.group) · \(rule.learned ? "自动学习" : "手动")")
                    .font(.caption).foregroundStyle(Theme.muted).textSelection(.enabled)
            } else { Text("无此规则").font(.caption).foregroundStyle(Theme.muted) }
        }.frame(maxWidth: .infinity, alignment: .leading)
    }
    private func choiceButton(_ title: String, selected: Bool, action: @escaping () -> Void) -> some View {
        Button(action: action) { Label(title, systemImage: selected ? "checkmark.circle.fill" : "circle") }
            .buttonStyle(StudioButtonStyle()).foregroundStyle(selected ? Theme.cyan : Theme.muted)
    }
    private func tint(_ kind: String) -> Color {
        kind == "新增" ? Theme.lime : kind == "删除" ? Theme.coral : kind == "修改" ? Theme.yellow : Theme.cyan
    }
    private func copyDiff() {
        let sections = [("我的修改", review.localChanges), ("外部更新", review.externalChanges)]
        let text = sections.map { title, changes in
            title + "\n" + changes.map { change in
                "[\(change.kind)] \(change.title) \(change.details)\n- \(change.before.map { serializeRuleLines([$0]) } ?? "无")\n+ \(change.after.map { serializeRuleLines([$0]) } ?? "无")"
            }.joined(separator: "\n\n")
        }.joined(separator: "\n\n")
        NSPasteboard.general.clearContents(); NSPasteboard.general.setString(text, forType: .string)
    }
}
