import SwiftUI

/// Shared visual primitives from the approved HTML design. Controls stay native
/// and accessible; their geometry and colors are consistent across all pages.
struct StudioButtonStyle: ButtonStyle {
    var primary = false
    @Environment(\.isEnabled) private var enabled
    func makeBody(configuration: Configuration) -> some View {
        configuration.label.font(.system(size: 12, weight: primary ? .semibold : .regular))
            .foregroundStyle(primary ? Theme.canvas : Theme.text)
            .padding(.horizontal, 12).padding(.vertical, 8)
            .background(primary ? Theme.cyan : Theme.panelRaised)
            .overlay(RoundedRectangle(cornerRadius: 7).stroke(primary ? .clear : Theme.border, lineWidth: 0.8))
            .clipShape(RoundedRectangle(cornerRadius: 7))
            .opacity(!enabled ? 0.4 : configuration.isPressed ? 0.7 : 1)
    }
}

struct StudioTabs: View {
    let title: String
    @Binding var selection: String
    let items: [(String, String, String)]
    var compact = false
    var body: some View {
        HStack(spacing: 4) {
            ForEach(items, id: \.0) { item in
                Button { selection = item.0 } label: {
                    HStack(spacing: 8) {
                        if !item.2.isEmpty { StudioIcon(item.2).frame(width: compact ? 15 : 17, height: compact ? 15 : 17) }
                        Text(item.1).font(.system(size: compact ? 12 : 13, weight: selection == item.0 ? .medium : .regular))
                    }.foregroundStyle(selection == item.0 ? Theme.text : Theme.muted)
                        .padding(.horizontal, compact ? 12 : 16).padding(.vertical, compact ? 7 : 12)
                        .background(selection == item.0 ? Theme.panelRaised : .clear)
                        .clipShape(RoundedRectangle(cornerRadius: compact ? 6 : 8))
                        .contentShape(Rectangle())
                }.buttonStyle(.plain).accessibilityLabel(item.1).accessibilityAddTraits(selection == item.0 ? [.isSelected] : [])
            }
        }.padding(compact ? 3 : 5).background(Theme.panel)
            .clipShape(RoundedRectangle(cornerRadius: compact ? 8 : 12))
    }
}

struct StudioTag: View {
    let text: String
    var tint = Theme.cyan
    var body: some View {
        Text(text).font(.system(size: 11)).foregroundStyle(tint)
            .padding(.horizontal, 9).padding(.vertical, 5)
            .background(tint.opacity(0.13)).clipShape(RoundedRectangle(cornerRadius: 6))
    }
}

struct StudioSelect: View {
    let title: String
    @Binding var selection: String
    let options: [(String, String)]
    @State private var expanded = false
    @Environment(\.isEnabled) private var enabled
    var body: some View {
        Button { expanded = true } label: {
            HStack {
                Text(options.first { $0.0 == selection }?.1 ?? selection).lineLimit(1)
                Spacer(minLength:8)
                Image(systemName:"chevron.down").font(.system(size:10)).foregroundStyle(Theme.muted)
            }.font(.system(size:12)).foregroundStyle(Theme.text).padding(10)
                .background(Theme.panelRaised)
                .overlay(RoundedRectangle(cornerRadius:7).stroke(Theme.border,lineWidth:0.8))
                .clipShape(RoundedRectangle(cornerRadius:7)).contentShape(Rectangle())
                .opacity(enabled ? 1:0.5)
        }.buttonStyle(.plain).accessibilityLabel(title).accessibilityValue(options.first{$0.0 == selection}?.1 ?? selection)
            .popover(isPresented:$expanded,arrowEdge:.bottom) {
                VStack(alignment:.leading,spacing:2) {
                    ForEach(options,id:\.0) { option in
                        Button { selection=option.0;expanded=false } label: {
                            HStack { Text(option.1);Spacer();if selection == option.0 {Image(systemName:"checkmark").foregroundStyle(Theme.cyan)} }
                                .font(.system(size:12)).padding(9).frame(minWidth:150)
                                .background(selection == option.0 ? Theme.soft:.clear).cornerRadius(5)
                                .contentShape(Rectangle())
                        }.buttonStyle(.plain)
                    }
                }.padding(6).foregroundStyle(Theme.text).background(Theme.panel)
            }
    }
}

/// Bare, consistent outline icons. Active traffic changes the icon's stroke and
/// small signal marks, rather than drawing a rectangular glow around it.
struct StudioIcon: View {
    let name: String
    var active = false
    init(_ name: String, active: Bool = false) { self.name = name; self.active = active }
    func outline(in rect: CGRect) -> Path {
        StudioSymbol(name: name, active: active).path(in: rect)
    }
    var body: some View {
        StudioSymbol(name: name, active: active)
            .stroke(style: StrokeStyle(lineWidth: 1.55, lineCap: .round, lineJoin: .round))
            .aspectRatio(1, contentMode: .fit).accessibilityHidden(true)
    }
}

private struct StudioSymbol: Shape {
    let name: String
    let active: Bool
    func path(in rect: CGRect) -> Path {
        var p = Path()
        func line(_ points: [(Double,Double)]) {
            guard let first = points.first else { return }
            p.move(to: CGPoint(x:first.0,y:first.1))
            for pt in points.dropFirst() { p.addLine(to:CGPoint(x:pt.0,y:pt.1)) }
        }
        func box(_ x:Double,_ y:Double,_ w:Double,_ h:Double,_ r:Double = 1.5) {
            p.addRoundedRect(in:CGRect(x:x,y:y,width:w,height:h),cornerSize:CGSize(width:r,height:r))
        }
        func circle(_ x:Double,_ y:Double,_ r:Double) { p.addEllipse(in:CGRect(x:x-r,y:y-r,width:r*2,height:r*2)) }
        switch name {
        case "wifi":
            for (y,w) in [(5.0,10.0),(9.0,7.0),(13.0,4.0)] {
                p.move(to:CGPoint(x:12-w,y:y+3));p.addQuadCurve(to:CGPoint(x:12+w,y:y+3),control:CGPoint(x:12,y:y-3))
            }
            circle(12,20,0.45)
        case "wifi.router", "gateway-wifi":
            box(3,15,18,6); circle(7,18,0.3);circle(11,18,0.3)
            p.move(to:CGPoint(x:7,y:7));p.addQuadCurve(to:CGPoint(x:17,y:7),control:CGPoint(x:12,y:2))
            p.move(to:CGPoint(x:9,y:10));p.addQuadCurve(to:CGPoint(x:15,y:10),control:CGPoint(x:12,y:7))
            circle(12,13,0.3)
        case "cable.connector", "ethernet", "server.rack":
            line([(4,4),(20,4),(20,15),(15,20),(9,20),(4,15),(4,4)])
            for x in [8.0,12.0,16.0] { line([(x,7),(x,10)]) }
        case "iphone", "smartphone": box(6,2,12,20,2);circle(12,18,0.35)
        case "ipad.landscape": box(2,5,20,14,2);circle(19,12,0.35)
        case "gamecontroller.fill", "gamecontroller", "switch":
            p.move(to:CGPoint(x:6,y:6));p.addLine(to:CGPoint(x:18,y:6))
            p.addQuadCurve(to:CGPoint(x:21,y:9),control:CGPoint(x:20,y:6))
            p.addLine(to:CGPoint(x:23,y:18));p.addQuadCurve(to:CGPoint(x:19,y:20),control:CGPoint(x:23,y:22))
            line([(19,20),(15,16),(9,16),(5,20)])
            p.move(to:CGPoint(x:5,y:20));p.addQuadCurve(to:CGPoint(x:1,y:18),control:CGPoint(x:0,y:22))
            p.addLine(to:CGPoint(x:3,y:9));p.addQuadCurve(to:CGPoint(x:6,y:6),control:CGPoint(x:4,y:6))
            line([(6,11),(10,11)]);line([(8,9),(8,13)]);circle(16,10,0.3);circle(19,13,0.3)
        case "desktopcomputer", "laptopcomputer", "desktopcomputer.and.macbook":
            box(2,3,20,14,1.5);line([(12,17),(12,21)]);line([(8,21),(16,21)])
        case "cloud":
            p.move(to:CGPoint(x:7,y:19));p.addCurve(to:CGPoint(x:5,y:8),control1:CGPoint(x:-1,y:18),control2:CGPoint(x:0,y:9))
            p.addCurve(to:CGPoint(x:17,y:7),control1:CGPoint(x:6,y:0),control2:CGPoint(x:15,y:0))
            p.addCurve(to:CGPoint(x:20,y:19),control1:CGPoint(x:25,y:6),control2:CGPoint(x:26,y:19))
            p.addLine(to:CGPoint(x:7,y:19))
        case "globe", "globe.americas":
            circle(12,12,10);line([(2,12),(22,12)])
            p.move(to:CGPoint(x:12,y:2));p.addCurve(to:CGPoint(x:12,y:22),control1:CGPoint(x:5,y:7),control2:CGPoint(x:5,y:17))
            p.addCurve(to:CGPoint(x:12,y:2),control1:CGPoint(x:19,y:17),control2:CGPoint(x:19,y:7))
        case "nosign", "ban": circle(12,12,10);line([(5,5),(19,19)])
        case "slider.horizontal.3", "settings":
            for (y,x) in [(5.0,9.0),(12.0,16.0),(19.0,8.0)] {
                line([(2,y),(x-2,y)]);line([(x+2,y),(22,y)]);line([(x,y-3),(x,y+3)])
            }
        case "arrow.triangle.branch", "split":
            line([(12,22),(12,13),(5,6),(2,6)]);line([(12,13),(19,6),(22,6)])
            line([(2,11),(2,6),(7,6)]);line([(17,6),(22,6),(22,11)])
        case "sparkles":
            line([(13,3),(15,10),(22,12),(15,14),(13,21),(11,14),(4,12),(11,10),(13,3)])
            line([(4,2),(4,6)]);line([(2,4),(6,4)])
        case "route":
            circle(4,5,2);circle(20,19,2);line([(7,5),(18,5)])
            p.move(to:CGPoint(x:18,y:5));p.addCurve(to:CGPoint(x:18,y:12),control1:CGPoint(x:23,y:5),control2:CGPoint(x:23,y:12))
            p.addLine(to:CGPoint(x:6,y:12));p.addCurve(to:CGPoint(x:6,y:19),control1:CGPoint(x:1,y:12),control2:CGPoint(x:1,y:19));p.addLine(to:CGPoint(x:17,y:19))
        case "activity": line([(1,12),(6,12),(9,3),(14,21),(17,12),(23,12)])
        case "list.bullet.rectangle", "list.bullet.rectangle.portrait": box(4,2,16,20);for y in [7.0,12.0,17.0] {line([(8,y),(16,y)])}
        case "arrow.clockwise":
            p.addArc(center:CGPoint(x:12,y:12),radius:8,startAngle:.degrees(-60),endAngle:.degrees(240),clockwise:false);line([(20,4),(20,10),(14,10)])
        default:
            box(9,2,6,5,1);line([(12,7),(12,12),(4,12),(4,16)]);line([(12,12),(20,12),(20,16)])
            box(1,16,6,6,1);box(17,16,6,6,1)
        }
        if active && name != "wifi" && name != "wifi.router" { circle(21,3,0.8) }
        let side = min(rect.width, rect.height)
        return p.applying(CGAffineTransform(scaleX:side/24,y:side/24)).offsetBy(dx:rect.midX-side/2,dy:rect.midY-side/2)
    }
}

struct StudioFieldStyle: TextFieldStyle {
    func _body(configuration: TextField<Self._Label>) -> some View {
        configuration.textFieldStyle(.plain).font(.system(size:12)).padding(10).background(Theme.panelRaised)
            .overlay(RoundedRectangle(cornerRadius:7).stroke(Theme.border,lineWidth:0.8))
            .clipShape(RoundedRectangle(cornerRadius:7))
    }
}
