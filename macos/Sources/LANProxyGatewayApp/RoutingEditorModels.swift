import Foundation

struct RoutingEditorFilter: Equatable {
    var query = ""
    var action = "all"
    var origin = "all"

    var search: String { query.trimmingCharacters(in: .whitespacesAndNewlines) }
    var isActive: Bool { !search.isEmpty || action != "all" || origin != "all" }

    func matches(_ rule: RoutingRule, group: String) -> Bool {
        guard action == "all" || rule.action == action else { return false }
        guard origin == "all" || (origin == "learned" ? rule.learned : !rule.learned) else { return false }
        return search.isEmpty || group.localizedCaseInsensitiveContains(search) ||
            rule.value.localizedCaseInsensitiveContains(search) || rule.type.localizedCaseInsensitiveContains(search)
    }
}

struct RoutingEditorPage {
    struct Entry: Identifiable {
        let id: UUID
        let ruleIndex: Int
    }

    let entries: [Entry]
    let number: Int
    let pageCount: Int
    let total: Int
    let pageSize: Int
    let canDragReorder: Bool

    var firstRecord: Int { total == 0 ? 0 : number * pageSize + 1 }
    var lastRecord: Int { total == 0 ? 0 : firstRecord + entries.count - 1 }
}

struct RoutingEditorProjection {
    let visibleGroupIndices: [Int]
    let ruleIndices: [[Int]]
    let matchedCount: Int
    let totalCount: Int
    let filter: RoutingEditorFilter

    init(groups: [(name: String, rules: [RoutingRule])], filter: RoutingEditorFilter) {
        self.filter = filter
        let indices = groups.map { group in
            group.rules.indices.filter { filter.matches(group.rules[$0], group: group.name) }
        }
        ruleIndices = indices
        visibleGroupIndices = groups.indices.filter {
            !indices[$0].isEmpty || (!filter.isActive && groups[$0].rules.isEmpty)
        }
        matchedCount = indices.reduce(0) { $0 + $1.count }
        totalCount = groups.reduce(0) { $0 + $1.rules.count }
    }

    func page(groupIndex: Int, rules: [RoutingRule], number: Int, pageSize: Int) -> RoutingEditorPage {
        let size = min(100, max(1, pageSize))
        let indices = ruleIndices.indices.contains(groupIndex) ? ruleIndices[groupIndex] : []
        let pages = indices.isEmpty ? 1 : (indices.count - 1) / size + 1
        let current = min(max(0, number), pages - 1)
        let entries = indices.dropFirst(current * size).prefix(size).compactMap { index -> RoutingEditorPage.Entry? in
            guard rules.indices.contains(index) else { return nil }
            return RoutingEditorPage.Entry(id: rules[index].id, ruleIndex: index)
        }
        return RoutingEditorPage(entries: entries, number: current, pageCount: pages, total: indices.count,
                                 pageSize: size, canDragReorder: !filter.isActive && pages == 1)
    }
}
