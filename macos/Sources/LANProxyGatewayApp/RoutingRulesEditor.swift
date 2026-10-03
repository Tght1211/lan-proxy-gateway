import AppKit
import Charts
import SwiftUI
import UniformTypeIdentifiers

struct RuleGroupDraft: Identifiable, Equatable {
    var id = UUID()
    var name: String // "" = ungrouped
    var rules: [RoutingRule]
    var collapsed = false
}

struct RuleGroupPreset {
    let name: String
    let rules: [RoutingRule]
}

let ruleGroupPresets: [RuleGroupPreset] = {
    guard let url = Bundle.main.url(forResource: "default_routing", withExtension: "json")
        ?? Bundle.module.url(forResource: "default_routing", withExtension: "json"),
          let data = try? Data(contentsOf: url),
          let rules = try? JSONDecoder().decode([RoutingRule].self, from: data) else { return [] }
    return makeGroupDrafts(rules).map { RuleGroupPreset(name: $0.name, rules: $0.rules) }
}()

func makeGroupDrafts(_ rules: [RoutingRule]) -> [RuleGroupDraft] {
    var order: [String] = []
    var bucket: [String: [RoutingRule]] = [:]
    for rule in rules {
        if bucket[rule.group] == nil {
            order.append(rule.group)
            bucket[rule.group] = []
        }
        bucket[rule.group]?.append(rule)
    }
    return order.map { RuleGroupDraft(name: $0, rules: bucket[$0] ?? []) }
}

func flattenGroups(_ groups: [RuleGroupDraft]) -> [RoutingRule] {
    groups.flatMap { group -> [RoutingRule] in
        let name = group.name.trimmingCharacters(in: .whitespaces)
        return group.rules.map { rule in
            var copy = rule
            copy.group = name
            return copy
        }
    }
}

struct RoutingRulesEditor: View {
    @EnvironmentObject private var model: AppModel
    @Environment(\.dismiss) private var dismiss
    @State private var groups: [RuleGroupDraft]
    @State private var isSaving = false
    @State private var saveError: String?
    @State private var editorMode = "list"
    @State private var text = ""
    @State private var parseNote: String?
    @State private var selectedGroupID: UUID?
    @State private var source = "all"
    @State private var baselineRules: [RoutingRule]
    @State private var configurationConflict = false

    private let embedded: Bool

    init(rules: [RoutingRule], embedded: Bool = false) {
        self.embedded = embedded
        _groups = State(initialValue: makeGroupDrafts(rules))
        _baselineRules = State(initialValue: rules)
    }

    private var flatRules: [RoutingRule] { flattenGroups(groups) }
    private var visibleIndices: [Int] {
        groups.indices.filter { source == "all" || groups[$0].rules.contains { $0.action == source } || groups[$0].rules.isEmpty }
    }
    private var selectedIndex: Int {
        guard let id = selectedGroupID, let idx = groups.firstIndex(where: { $0.id == id }), visibleIndices.contains(idx) else {
            return visibleIndices.first ?? -1
        }
        return idx
    }

    var body: some View {
        VStack(spacing: 0) {
            HStack(alignment: .top) {
                VStack(alignment: .leading, spacing: 3) {
                    Text("分流规则").font(.title3.weight(.semibold))
                    Text("所有接入方式共用规则；保存后即时生效。")
                        .font(.caption).foregroundStyle(Theme.cyan)
                    Text("设备策略优先；其他规则按全局顺序首条命中。")
                        .font(.caption).foregroundStyle(Theme.muted)
                }
                Spacer()
                StudioTabs(title: "编辑方式", selection: Binding(get: { editorMode }, set: { mode in
                    if mode == "text" { text = serializeRuleLines(flatRules); parseNote = nil; editorMode = mode }
                    else if syncTextToDraft() { editorMode = mode }
                }), items: [("list", "交互式", ""), ("text", "文本式", "")], compact:true)
                if editorMode == "list" {
                    Menu {
                        Section("预设分组 · 走上游代理") {
                            ForEach(ruleGroupPresets, id: \.name) { preset in
                                Button(preset.name) { addPreset(preset) }
                            }
                        }
                        Divider()
                        Button { addCustomGroup() } label: { Label("自定义分组", systemImage: "folder.badge.plus") }
                        Button { addUngroupedRule() } label: { Label("单条规则", systemImage: "plus") }
                    } label: {
                        Label("添加", systemImage: "plus")
                    }
                    .menuStyle(.borderedButton)
                    .fixedSize()
                }
            }
            .padding(.horizontal, 20).padding(.vertical, 16)
            .background(Theme.panel)
            Divider().overlay(Theme.border)

            HStack(spacing: 16) {
                StudioTabs(title: "规则动作筛选", selection:$source, items:[("all","全部规则",""),("proxy","代理出口","cloud"),("direct","本机直连","globe"),("reject","拒绝","ban")], compact:true)
                    .help("按规则动作筛选分组；混合分组保留其他动作的规则。筛选不改变全局顺序，保存时保留未显示的规则。")
                Spacer()
                Text("每条规则只指定一种动作").font(.caption).foregroundStyle(Theme.muted)
            }.padding(.horizontal, 20).padding(.vertical, 10)
            if editorMode == "text" {
                VStack(alignment: .leading, spacing: 8) {
					Text("每行一条：类型,值,动作。支持 DOMAIN / DOMAIN-SUFFIX / IP-CIDR / SRC-IP；DIRECT / REJECT，其他目标视为代理。SRC-IP 是设备级前置规则，优先于域名规则。「# == 分组: 名称 ==」行开始一个分组，「# == 未分组 ==」结束分组；其他 # 行为注释。")
                        .font(.caption).foregroundStyle(Theme.muted)
                        .fixedSize(horizontal: false, vertical: true)
                    TextEditor(text: $text)
                        .font(.system(size: 12, design: .monospaced))
                        .scrollContentBackground(.hidden)
                        .scrollIndicators(.hidden)
                        .padding(8)
                        .background(Theme.panel)
                        .overlay(RoundedRectangle(cornerRadius: Theme.radius).stroke(Theme.border, lineWidth: 0.8))
                        .clipShape(RoundedRectangle(cornerRadius: Theme.radius))
                    if let parseNote {
                        Text(parseNote).font(.caption2).foregroundStyle(Theme.yellow)
                    }
                }
                .padding(20)
            } else if groups.isEmpty {
                VStack(spacing: 12) {
                    Image(systemName: "arrow.triangle.branch").font(.system(size: 28)).foregroundStyle(Theme.muted)
                    Text("暂无规则，未配置的域名先直连").font(.callout).foregroundStyle(Theme.muted)
                    HStack(spacing: 8) {
                        ForEach(ruleGroupPresets.prefix(3), id: \.name) { preset in
                            Button("添加 \(preset.name)") { addPreset(preset) }.buttonStyle(StudioButtonStyle())
                        }
                    }
                    Button { addUngroupedRule() } label: { Label("添加单条规则", systemImage: "plus") }.buttonStyle(StudioButtonStyle())
                    Button { editorMode = "text" } label: { Label("粘贴文本规则", systemImage: "doc.on.clipboard") }.buttonStyle(.plain).font(.caption).foregroundStyle(Theme.cyan)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                HStack(spacing: 0) {
                    // 左侧分组目录
                    VStack(spacing: 0) {
                        HStack {
                            Text("分组").font(.caption.weight(.bold)).foregroundStyle(Theme.muted)
                            Spacer()
                            Menu {
                                Section("预设分组 · 走上游代理") {
                                    ForEach(ruleGroupPresets, id: \.name) { preset in
                                        Button(preset.name) { addPreset(preset); selectLastGroup() }
                                    }
                                }
                                Divider()
                                Button { addCustomGroup(); selectLastGroup() } label: { Label("自定义分组", systemImage: "folder.badge.plus") }
                                Button { addUngroupedRule(); selectUngrouped() } label: { Label("单条规则", systemImage: "plus") }
                            } label: {
                                Image(systemName: "plus.circle.fill").font(.system(size: 14)).foregroundStyle(Theme.cyan)
                            }
                            .menuStyle(.borderlessButton)
                            .menuIndicator(.hidden)
                        }
                        .padding(.horizontal, 12).padding(.vertical, 8)
                        Divider().overlay(Theme.border)
                        ScrollView(.vertical, showsIndicators: true) {
                            VStack(spacing: 2) {
                                ForEach(visibleIndices, id: \.self) { index in
                                    groupDirectoryRow(index: index, group: groups[index], selected: index == selectedIndex)
                                }
                            }
                            .padding(.horizontal, 8).padding(.vertical, 6)
                        }
                        .background(Theme.panel)
                    }
                    .frame(width: 190)
                    Divider().overlay(Theme.border)
                    // 右侧规则窗格
                    VStack(spacing: 0) {
                        if selectedIndex >= 0 {
                            groupPaneToolbar(groupIndex: selectedIndex)
                            Divider().overlay(Theme.border)
                            List {
                                ForEach(Array($groups[selectedIndex].rules.enumerated()), id: \.element.id) { ruleIndex, $rule in
                                    ruleRow(number: ruleIndex + 1, rule: $rule, groupIndex: selectedIndex)
                                        .listRowSeparator(.hidden)
                                        .listRowBackground(Color.clear)
                                        .listRowInsets(EdgeInsets(top: 3, leading: 20, bottom: 3, trailing: 20))
                                }
                                .onMove { indices, offset in
                                    groups[selectedIndex].rules.move(fromOffsets: indices, toOffset: offset)
                                }
                            }
                            .listStyle(.plain)
                            .scrollContentBackground(.hidden)
                            .scrollIndicators(.hidden)
                        } else {
                            Text("选择左侧分组查看规则").font(.callout).foregroundStyle(Theme.muted)
                                .frame(maxWidth: .infinity, maxHeight: .infinity)
                        }
                    }
                }
            }

            Divider().overlay(Theme.border)
            HStack(spacing: 8) {
                if let saveError {
                    Label(saveError, systemImage: "exclamationmark.triangle.fill")
                        .font(.caption).foregroundStyle(Theme.coral).lineLimit(2)
                } else {
                    RuleCountChip(label: "代理", count: count("proxy"), color: Theme.cyan)
                    RuleCountChip(label: "直连", count: count("direct"), color: Theme.lime)
                    RuleCountChip(label: "拒绝", count: count("reject"), color: Theme.coral)
                    if let duplicateRuleNotice {
                        Label(duplicateRuleNotice, systemImage: "exclamationmark.triangle")
                            .font(.caption2).foregroundStyle(Theme.yellow)
                            .lineLimit(1)
                            .help(duplicateRuleNotice + "。同一规则出现在多个位置时，只有排在最前面的一条生效，建议删除多余的。")
                    }
                }
                Spacer()
                Button(embedded ? "撤销修改" : "取消") {
                    if embedded {
                        loadLatest(model.status?.routing ?? baselineRules)
                    } else { dismiss() }
                }.buttonStyle(StudioButtonStyle()).disabled(isSaving)
                Button {
                    save()
                } label: {
                    if isSaving {
                        ProgressView().controlSize(.small).frame(width: 56)
                    } else {
                        Text("应用规则")
                    }
                }
                .buttonStyle(ActionButtonStyle(tint: Theme.cyan))
                .disabled((editorMode == "list" && hasInvalidRule) || isSaving || model.isBusy || configurationConflict)
            }
            .padding(.horizontal, 20).padding(.vertical, 14)
            .background(Theme.panel)
        }
        .frame(minWidth: embedded ? 0 : 940, maxWidth: embedded ? .infinity : 940, minHeight: 600, maxHeight: embedded ? .infinity : 600).background(Theme.canvasBackground)
        .onReceive(model.$status) { status in
            guard !isSaving, let status else { return }
            let changed = editorMode == "text" ? text != serializeRuleLines(baselineRules) : NetworkRuleRevision.signature(flatRules) != NetworkRuleRevision.signature(baselineRules)
            switch NetworkRuleRevision.assess(baseline: baselineRules, latest: status.routing ?? [], draftChanged: changed) {
            case .unchanged: break
            case .reload: loadLatest(status.routing ?? [])
            case .conflict:
                configurationConflict = true
                saveError = "规则已在其他位置更新。请先复制草稿，再撤销修改并重新编辑，避免覆盖新规则。"
            }
        }
    }

    // MARK: group header (sidebar + pane)

    private func selectLastGroup() { selectedGroupID = groups.last?.id }
    private func selectUngrouped() {
        selectedGroupID = groups.first(where: { $0.name.isEmpty })?.id ?? groups.last?.id
    }

    private func groupDirectoryRow(index: Int, group: RuleGroupDraft, selected: Bool) -> some View {
        let summary = groupActionSummary(group)
        let color = summary == "mixed" ? Theme.yellow : actionColor(summary)
        return Button {
            selectedGroupID = group.id
        } label: {
            HStack(spacing: 8) {
                // 左侧优先级色条：选中时高亮，未选中时淡显
                RoundedRectangle(cornerRadius: 2)
                    .fill(selected ? color : color.opacity(0.35))
                    .frame(width: 3)
                Circle().fill(color).frame(width: 6, height: 6)
                VStack(alignment: .leading, spacing: 1) {
                    Text(group.name.isEmpty ? "未分组" : group.name)
                        .font(.system(size: 12, weight: selected ? .bold : .semibold))
                        .foregroundStyle(selected ? Color.primary : Color.primary.opacity(0.85))
                        .lineLimit(1)
                    Text("\(group.rules.count) 条 · \(groupActionLabel(group))")
                        .font(.system(size: 9)).foregroundStyle(Theme.muted)
                }
                Spacer()
            }
            .padding(.horizontal, 8).padding(.vertical, 7)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(selected ? color.opacity(0.10) : Color.clear)
            .clipShape(RoundedRectangle(cornerRadius: 6))
        }
        .buttonStyle(.plain)
        .help("优先级 \(index + 1)：组顺序即匹配优先级块")
    }

    private func groupActionLabel(_ group: RuleGroupDraft) -> String {
        let s = groupActionSummary(group)
        return s == "mixed" ? "混合" : (s == "proxy" ? "代理" : (s == "direct" ? "直连" : "拒绝"))
    }

    private func groupPaneToolbar(groupIndex: Int) -> some View {
        guard groups.indices.contains(groupIndex) else { return AnyView(EmptyView()) }
        let group = groups[groupIndex]
        let count = group.rules.count
        let first = ruleNumber(groupIndex: groupIndex, ruleIndex: 0)
        let globalRange = count > 0 ? "全局 #\(first)\(count > 1 ? "–\(first + count - 1)" : "")" : ""
        return AnyView(
            HStack(spacing: 10) {
                TextField("分组名", text: $groups[groupIndex].name)
                    .textFieldStyle(.plain)
                    .font(.system(size: 13, weight: .bold))
                    .frame(maxWidth: 200)
                Text("\(count) 条").font(.caption).foregroundStyle(Theme.muted)
                if !globalRange.isEmpty {
                    Text(globalRange).font(.caption2).foregroundStyle(Theme.muted.opacity(0.8))
                        .help("该分组在全局匹配顺序中的位置；组顺序即优先级块")
                }
                Spacer()
                HStack(spacing: 6) {
                    Circle().fill(groupActionSummaryColor(group)).frame(width: 7, height: 7)
                    StudioSelect(title:"分组动作",selection:groupActionBinding(groupIndex),options:(groupActionSummary(group) == "mixed" ? [("mixed","混合")]:[])+[("proxy","上游代理"),("direct","本机直连"),("reject","拒绝")]).frame(width:130)
                }
                Button {
                    groups[groupIndex].rules.append(RoutingRule(type: "domain-suffix", value: "", action: dominantAction(groups[groupIndex])))
                } label: { Image(systemName: "plus").font(.system(size: 11, weight: .bold)) }
                .buttonStyle(.plain).foregroundStyle(Theme.cyan).help("向此分组添加规则")
                Button { guard groupIndex > 0 else { return }; groups.swapAt(groupIndex, groupIndex - 1) } label: {
                    Image(systemName: "arrow.up").font(.system(size: 10))
                        .foregroundStyle(groupIndex > 0 ? Theme.muted : Theme.muted.opacity(0.3))
                }
                .buttonStyle(.plain).disabled(groupIndex == 0).help("上移分组（提高优先级）")
                Button { guard groupIndex < groups.count - 1 else { return }; groups.swapAt(groupIndex, groupIndex + 1) } label: {
                    Image(systemName: "arrow.down").font(.system(size: 10))
                        .foregroundStyle(groupIndex < groups.count - 1 ? Theme.muted : Theme.muted.opacity(0.3))
                }
                .buttonStyle(.plain).disabled(groupIndex == groups.count - 1).help("下移分组（降低优先级）")
                Button {
                    let id = groups[groupIndex].id
                    groups.remove(at: groupIndex)
                    selectedGroupID = groups.first?.id
                    _ = id
                } label: { Image(systemName: "trash").font(.system(size: 11)).foregroundStyle(Theme.coral.opacity(0.8)) }
                .buttonStyle(.plain).help("删除整个分组及其规则")
            }
            .padding(.horizontal, 20).padding(.vertical, 10)
            .background(Theme.panel)
        )
    }

    @ViewBuilder
    private func groupHeader(groupIndex: Int, group: Binding<RuleGroupDraft>) -> some View {
        HStack(spacing: 8) {
            Button {
                group.wrappedValue.collapsed.toggle()
            } label: {
                Image(systemName: group.wrappedValue.collapsed ? "chevron.right" : "chevron.down")
                    .font(.system(size: 10, weight: .bold))
                    .foregroundStyle(Theme.muted)
                    .frame(width: 16)
            }
            .buttonStyle(.plain)
            .help(group.wrappedValue.collapsed ? "展开分组" : "折叠分组")
            if group.wrappedValue.name.isEmpty {
                Text("未分组")
                    .font(.system(size: 12, weight: .bold))
                    .foregroundStyle(Theme.muted)
            } else {
                TextField("分组名", text: group.name)
                    .textFieldStyle(.plain)
                    .font(.system(size: 12, weight: .bold))
                    .frame(maxWidth: 160)
            }
            Text("\(group.wrappedValue.rules.count) 条")
                .font(.system(size: 10)).foregroundStyle(Theme.muted)
            Spacer()
            HStack(spacing: 6) {
                Circle().fill(groupActionSummaryColor(group.wrappedValue)).frame(width: 7, height: 7)
                Picker("组动作", selection: groupActionBinding(groupIndex)) {
                    if groupActionSummary(group.wrappedValue) == "mixed" {
                        Text("混合").tag("mixed")
                    }
                    Text("上游代理").tag("proxy")
                    Text("本机直连").tag("direct")
                    Text("拒绝").tag("reject")
                }
                .labelsHidden()
                .help("为该分组的全部规则统一设置动作")
            }
            .frame(width: 128)
            Button {
                groups[groupIndex].rules.append(RoutingRule(type: "domain-suffix", value: "", action: dominantAction(groups[groupIndex])))
                groups[groupIndex].collapsed = false
            } label: {
                Image(systemName: "plus").font(.system(size: 10, weight: .bold)).foregroundStyle(Theme.cyan)
            }
            .buttonStyle(.plain).help("向此分组添加规则")
            Button {
                guard groupIndex > 0 else { return }
                groups.swapAt(groupIndex, groupIndex - 1)
            } label: {
                Image(systemName: "arrow.up").font(.system(size: 10)).foregroundStyle(groupIndex > 0 ? Theme.muted : Theme.muted.opacity(0.3))
            }
            .buttonStyle(.plain).disabled(groupIndex == 0).help("上移分组（提高优先级）")
            Button {
                guard groupIndex < groups.count - 1 else { return }
                groups.swapAt(groupIndex, groupIndex + 1)
            } label: {
                Image(systemName: "arrow.down").font(.system(size: 10)).foregroundStyle(groupIndex < groups.count - 1 ? Theme.muted : Theme.muted.opacity(0.3))
            }
            .buttonStyle(.plain).disabled(groupIndex == groups.count - 1).help("下移分组（降低优先级）")
            Button {
                groups.remove(at: groupIndex)
            } label: {
                Image(systemName: "trash").font(.system(size: 10)).foregroundStyle(Theme.coral.opacity(0.8))
            }
            .buttonStyle(.plain).help("删除整个分组及其规则")
        }
        .padding(.horizontal, 4)
    }

    // MARK: rule row

    @ViewBuilder
    private func ruleRow(number: Int, rule: Binding<RoutingRule>, groupIndex: Int) -> some View {
        HStack(spacing: 10) {
            Text("\(number)")
                .font(.system(size: 10, weight: .bold, design: .monospaced))
                .foregroundStyle(Theme.muted)
                .frame(width: 22, height: 20)
                .background(Theme.panelRaised)
                .clipShape(RoundedRectangle(cornerRadius: 4))
            StudioSelect(title:"匹配类型",selection:rule.type,options:[("domain","完整域名"),("domain-suffix","域名后缀"),("ip-cidr","IP-CIDR"),("src-ip","设备 IP")]).frame(width:120)
            TextField(placeholder(for: rule.wrappedValue.type), text: rule.value)
                .textFieldStyle(DarkFieldStyle())
                .font(.system(size: 12, design: .monospaced))
            if rule.wrappedValue.learned {
                Text("学习")
                    .font(.system(size: 8, weight: .bold))
                    .foregroundStyle(Theme.yellow)
                    .padding(.horizontal, 5).padding(.vertical, 2)
                    .background(Theme.yellow.opacity(0.14))
                    .clipShape(Capsule())
                    .help("根据收到的有效响应自动学习；明确规则优先，可随时删除")
            }
            actionPill(rule.action)
            Menu {
                Section("移动到分组") {
                    ForEach(Array(groups.enumerated()), id: \.element.id) { targetIndex, target in
                        if targetIndex != groupIndex {
                            Button(target.name.isEmpty ? "未分组" : target.name) {
                                moveRule(rule.wrappedValue.id, from: groupIndex, to: targetIndex)
                            }
                        }
                    }
                    Button("新建分组…") { moveRuleToNewGroup(rule.wrappedValue.id, from: groupIndex) }
                }
                Divider()
                Button(role: .destructive) {
                    let id = rule.wrappedValue.id
                    groups[groupIndex].rules.removeAll { $0.id == id }
                } label: { Label("删除规则", systemImage: "trash") }
            } label: {
                Image(systemName: "ellipsis")
                    .font(.system(size: 11, weight: .bold))
                    .foregroundStyle(Theme.muted)
                    .frame(width: 22)
            }
            .menuStyle(.borderlessButton)
            .menuIndicator(.hidden)
            .help("更多操作：移动到分组或删除")
        }
        .padding(.horizontal, 10).padding(.vertical, 8)
        .background(Theme.panel)
        .overlay(RoundedRectangle(cornerRadius: Theme.radius).stroke(Theme.border, lineWidth: 0.8))
        .clipShape(RoundedRectangle(cornerRadius: Theme.radius))
    }

    @ViewBuilder
    private func actionPill(_ action: Binding<String>) -> some View {
        let value = action.wrappedValue
        let color = actionColor(value)
        StudioSelect(title:"规则出口",selection:action,options:[("proxy","代理"),("direct","直连"),("reject","拒绝")])
            .frame(width:100).foregroundStyle(color)
        .help("选择该规则的出口动作")
    }

    // MARK: group helpers

    private func ruleNumber(groupIndex: Int, ruleIndex: Int) -> Int {
        groups.prefix(groupIndex).reduce(0) { $0 + $1.rules.count } + ruleIndex + 1
    }

    private func groupActionSummary(_ group: RuleGroupDraft) -> String {
        let actions = Set(group.rules.map(\.action))
        return actions.count == 1 ? (actions.first ?? "proxy") : "mixed"
    }

    private func groupActionSummaryColor(_ group: RuleGroupDraft) -> Color {
        let summary = groupActionSummary(group)
        return summary == "mixed" ? Theme.yellow : actionColor(summary)
    }

    private func dominantAction(_ group: RuleGroupDraft) -> String {
        let summary = groupActionSummary(group)
        return summary == "mixed" ? "proxy" : summary
    }

    private func groupActionBinding(_ groupIndex: Int) -> Binding<String> {
        Binding(
            get: {
                guard groups.indices.contains(groupIndex) else { return "proxy" }
                return groupActionSummary(groups[groupIndex])
            },
            set: { newValue in
                guard newValue != "mixed", groups.indices.contains(groupIndex) else { return }
                for i in groups[groupIndex].rules.indices {
                    groups[groupIndex].rules[i].action = newValue
                }
            }
        )
    }

    private func addPreset(_ preset: RuleGroupPreset) {
        let existing = Set(flatRules.map { "\($0.type)|\($0.value.lowercased())" })
        let newRules = preset.rules.filter { !existing.contains("\($0.type)|\($0.value.lowercased())") }
        guard !newRules.isEmpty else { return }
        if let index = groups.firstIndex(where: { $0.name == preset.name }) {
            groups[index].rules.append(contentsOf: newRules)
            groups[index].collapsed = false
        } else {
            groups.append(RuleGroupDraft(name: preset.name, rules: newRules))
        }
    }

    private func addCustomGroup() {
        var name = "新分组"
        var counter = 2
        while groups.contains(where: { $0.name == name }) {
            name = "新分组 \(counter)"
            counter += 1
        }
        groups.append(RuleGroupDraft(name: name, rules: [RoutingRule(type: "domain-suffix", value: "", action: "proxy")]))
    }

    private func addUngroupedRule() {
        if let index = groups.firstIndex(where: { $0.name.isEmpty }) {
            groups[index].rules.append(RoutingRule(type: "domain-suffix", value: "", action: "proxy"))
            groups[index].collapsed = false
        } else {
            groups.append(RuleGroupDraft(name: "", rules: [RoutingRule(type: "domain-suffix", value: "", action: "proxy")]))
        }
    }

    private func moveRule(_ id: UUID, from sourceIndex: Int, to targetIndex: Int) {
        guard groups.indices.contains(sourceIndex), groups.indices.contains(targetIndex),
              let ruleIndex = groups[sourceIndex].rules.firstIndex(where: { $0.id == id }) else { return }
        let rule = groups[sourceIndex].rules.remove(at: ruleIndex)
        groups[targetIndex].rules.append(rule)
        groups[targetIndex].collapsed = false
        if groups[sourceIndex].rules.isEmpty && groups[sourceIndex].name.isEmpty {
            groups.remove(at: sourceIndex)
        }
    }

    private func moveRuleToNewGroup(_ id: UUID, from sourceIndex: Int) {
        guard groups.indices.contains(sourceIndex),
              let ruleIndex = groups[sourceIndex].rules.firstIndex(where: { $0.id == id }) else { return }
        let rule = groups[sourceIndex].rules.remove(at: ruleIndex)
        var name = "新分组"
        var counter = 2
        while groups.contains(where: { $0.name == name }) {
            name = "新分组 \(counter)"
            counter += 1
        }
        groups.append(RuleGroupDraft(name: name, rules: [rule]))
        if groups[sourceIndex].rules.isEmpty && groups[sourceIndex].name.isEmpty {
            groups.remove(at: sourceIndex)
        }
    }

    // duplicateRuleNotice lists rules that appear more than once (same type +
    // value) so users can clean up cross-group duplicates.
    private var duplicateRuleNotice: String? {
        var seen: [String: [String]] = [:]
        for group in groups {
            let groupLabel = group.name.isEmpty ? "未分组" : group.name
            for rule in group.rules {
                let value = rule.value.trimmingCharacters(in: .whitespaces).lowercased()
                guard !value.isEmpty else { continue }
                seen["\(rule.type),\(value)", default: []].append(groupLabel)
            }
        }
        let duplicates = seen.filter { $0.value.count > 1 }
        guard !duplicates.isEmpty else { return nil }
        let details = duplicates
            .sorted { $0.key < $1.key }
            .prefix(3)
            .map { key, places -> String in
                let value = key.split(separator: ",").last.map(String.init) ?? key
                return "\(value)（\(places.joined(separator: " / "))）"
            }
        let suffix = duplicates.count > 3 ? " 等 \(duplicates.count) 条" : ""
        return "重复规则：\(details.joined(separator: "；"))\(suffix)"
    }

    // MARK: save / text sync

    private func loadLatest(_ rules: [RoutingRule]) {
        baselineRules = rules
        groups = makeGroupDrafts(rules)
        text = serializeRuleLines(rules)
        configurationConflict = false; saveError = nil; parseNote = nil
    }

    private func save() {
        guard let status = model.status else {
            saveError = "当前配置不可用，请刷新后重试。"; return
        }
        let latest = status.routing ?? []
        guard NetworkRuleRevision.signature(latest) == NetworkRuleRevision.signature(baselineRules) else {
            configurationConflict = true
            saveError = "规则已更新，请撤销修改后重新编辑。"; return
        }
        if editorMode == "text", !syncTextToDraft() { return }
        saveError = nil
        isSaving = true
        let rules = flattenGroups(groups)
        Task {
            let ok = await model.applyRoutingRules(rules)
            isSaving = false
            if ok {
                loadLatest(model.status?.routing ?? rules)
                if !embedded { dismiss() }
            } else {
                saveError = model.errorMessage ?? "保存失败，请检查规则"
            }
        }
    }

    private var ruleTemplateText: String {
        """
        # 每行一条规则：类型,值,动作（删掉行首 # 即可启用）
        # 动作：PROXY 走上游代理 / DIRECT 本机直连 / REJECT 拒绝
        # 「# == 分组: 名称 ==」开始一个分组，「# == 未分组 ==」结束分组

        # == 分组: YouTube ==
        # DOMAIN-SUFFIX,youtube.com,PROXY
        # DOMAIN-SUFFIX,googlevideo.com,PROXY

        # == 分组: 国内直连 ==
        # DOMAIN-SUFFIX,bilibili.com,DIRECT
        # DOMAIN-SUFFIX,qq.com,DIRECT

        # == 未分组 ==
        # DOMAIN-SUFFIX,doubleclick.net,REJECT
        # IP-CIDR,203.0.113.0/24,REJECT
        """
    }

    @discardableResult
    private func syncTextToDraft() -> Bool {
        let result = parseRuleLines(text)
        guard result.skipped.isEmpty else {
            parseNote = "无法识别 \(result.skipped.count) 行：\(result.skipped.prefix(3).joined(separator: "；"))。请修正后再应用。"
            saveError = parseNote
            return false
        }
        groups = makeGroupDrafts(result.rules)
        parseNote = nil; saveError = nil
        return true
    }

    private func placeholder(for type: String) -> String {
		if type == "ip-cidr" { return "例如 192.168.1.0/24" }
		if type == "src-ip" { return "例如 192.168.1.50" }
		return "例如 openai.com"
    }

    private func actionColor(_ action: String) -> Color {
        switch action {
        case "proxy": return Theme.cyan
        case "reject": return Theme.coral
        default: return Theme.lime
        }
    }

    private var hasInvalidRule: Bool { flatRules.contains { $0.value.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty } }
    private func count(_ action: String) -> Int { flatRules.filter { $0.action == action }.count }
}

struct RuleCountChip: View {
    let label: String
    let count: Int
    let color: Color
    var body: some View {
        HStack(spacing: 5) {
            Circle().fill(color).frame(width: 6, height: 6)
            Text("\(label) \(count)")
        }
        .font(.system(size: 10, weight: .semibold))
        .foregroundStyle(color)
        .padding(.horizontal, 8).frame(height: 22)
        .background(color.opacity(0.09))
        .clipShape(RoundedRectangle(cornerRadius: 5))
    }
}
