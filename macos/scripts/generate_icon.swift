import AppKit
import Foundation

guard CommandLine.arguments.count == 2 else {
    fputs("usage: generate_icon.swift OUTPUT.png\n", stderr)
    exit(2)
}

let size = NSSize(width: 1024, height: 1024)
let image = NSImage(size: size)
image.lockFocus()

// Squircle with a deep teal→emerald gradient, dock-icon style: one bold
// white glyph on a saturated field, legible at 16 px.
let squircle = NSBezierPath(roundedRect: NSRect(x: 72, y: 72, width: 880, height: 880), xRadius: 196, yRadius: 196)
let top = NSColor(calibratedRed: 0.088, green: 0.63, blue: 0.50, alpha: 1)
let bottom = NSColor(calibratedRed: 0.043, green: 0.345, blue: 0.30, alpha: 1)
NSGradient(starting: top, ending: bottom)!.draw(in: squircle, angle: -90)

// Soft top sheen, like Big Sur system icons.
NSGraphicsContext.current?.saveGraphicsState()
squircle.addClip()
let sheen = NSGradient(starting: NSColor(calibratedWhite: 1.0, alpha: 0.16), ending: NSColor(calibratedWhite: 1.0, alpha: 0.0))!
sheen.draw(in: NSRect(x: 72, y: 512, width: 880, height: 440), angle: -90)
NSGraphicsContext.current?.restoreGraphicsState()


let white = NSColor(calibratedWhite: 1.0, alpha: 1.0)

func stroke(_ path: NSBezierPath, width: CGFloat) {
    path.lineWidth = width
    path.lineCapStyle = .round
    path.lineJoinStyle = .round
    white.setStroke()
    path.stroke()
}

// Entry line → hub node → two branches with arrowheads.
let hub = NSPoint(x: 452, y: 512)
let strokeWidth: CGFloat = 92

let entry = NSBezierPath()
entry.move(to: NSPoint(x: 236, y: 512))
entry.line(to: hub)
stroke(entry, width: strokeWidth)

let branchLen: CGFloat = 268
let angles: [CGFloat] = [.pi / 5.4, -.pi / 5.4]
for angle in angles {
    let dir = NSPoint(x: cos(angle), y: sin(angle))
    let end = NSPoint(x: hub.x + dir.x * branchLen, y: hub.y + dir.y * branchLen)
    let branch = NSBezierPath()
    branch.move(to: hub)
    branch.line(to: end)
    stroke(branch, width: strokeWidth)

    // Rounded arrowhead: stroked+filled triangle pointing along the branch.
    let tip = NSPoint(x: end.x + dir.x * 118, y: end.y + dir.y * 118)
    let perp = NSPoint(x: -dir.y, y: dir.x)
    let backX = tip.x - dir.x * 148
    let backY = tip.y - dir.y * 148
    let head = NSBezierPath()
    head.move(to: tip)
    head.line(to: NSPoint(x: backX + perp.x * 104, y: backY + perp.y * 104))
    head.line(to: NSPoint(x: backX - perp.x * 104, y: backY - perp.y * 104))
    head.close()
    white.setFill()
    head.lineWidth = 44
    head.lineJoinStyle = .round
    white.setStroke()
    head.stroke()
    head.fill()
}

// Hub node: solid white circle with a gradient-colored core dot.
let hubRadius: CGFloat = 108
white.setFill()
NSBezierPath(ovalIn: NSRect(x: hub.x - hubRadius, y: hub.y - hubRadius, width: hubRadius * 2, height: hubRadius * 2)).fill()
bottom.setFill()
NSBezierPath(ovalIn: NSRect(x: hub.x - 42, y: hub.y - 42, width: 84, height: 84)).fill()

image.unlockFocus()
guard let tiff = image.tiffRepresentation,
      let bitmap = NSBitmapImageRep(data: tiff),
      let png = bitmap.representation(using: .png, properties: [:]) else {
    fputs("failed to render icon\n", stderr)
    exit(1)
}
try png.write(to: URL(fileURLWithPath: CommandLine.arguments[1]))
