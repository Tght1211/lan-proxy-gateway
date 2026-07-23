import AppKit
import Foundation

guard CommandLine.arguments.count == 2 else {
    fputs("usage: generate_icon.swift OUTPUT.png\n", stderr)
    exit(2)
}

let size = NSSize(width: 1024, height: 1024)
let image = NSImage(size: size)
image.lockFocus()

let background = NSBezierPath(roundedRect: NSRect(x: 72, y: 72, width: 880, height: 880), xRadius: 190, yRadius: 190)
NSColor(calibratedRed: 0.10, green: 0.12, blue: 0.14, alpha: 1).setFill()
background.fill()

let cyan = NSColor(calibratedRed: 0.20, green: 0.78, blue: 0.78, alpha: 1)
let coral = NSColor(calibratedRed: 0.96, green: 0.42, blue: 0.34, alpha: 1)
let white = NSColor(calibratedWhite: 0.96, alpha: 1)

func line(from: NSPoint, to: NSPoint, color: NSColor) {
    let path = NSBezierPath()
    path.move(to: from)
    path.line(to: to)
    path.lineWidth = 38
    path.lineCapStyle = .round
    color.setStroke()
    path.stroke()
}

func node(center: NSPoint, radius: CGFloat, color: NSColor) {
    let rect = NSRect(x: center.x - radius, y: center.y - radius, width: radius * 2, height: radius * 2)
    color.setFill()
    NSBezierPath(ovalIn: rect).fill()
}

let center = NSPoint(x: 512, y: 512)
let points = [
    NSPoint(x: 300, y: 710),
    NSPoint(x: 724, y: 710),
    NSPoint(x: 300, y: 314),
    NSPoint(x: 724, y: 314)
]
for (index, point) in points.enumerated() {
    line(from: center, to: point, color: index.isMultiple(of: 2) ? cyan : coral)
    node(center: point, radius: 66, color: index.isMultiple(of: 2) ? cyan : coral)
}

let gateway = NSBezierPath(roundedRect: NSRect(x: 374, y: 396, width: 276, height: 232), xRadius: 58, yRadius: 58)
white.setFill()
gateway.fill()

NSColor(calibratedRed: 0.10, green: 0.12, blue: 0.14, alpha: 1).setFill()
for x in [438.0, 512.0, 586.0] {
    NSBezierPath(ovalIn: NSRect(x: x - 18, y: 480, width: 36, height: 36)).fill()
}

image.unlockFocus()
guard let tiff = image.tiffRepresentation,
      let bitmap = NSBitmapImageRep(data: tiff),
      let png = bitmap.representation(using: .png, properties: [:]) else {
    fputs("failed to render icon\n", stderr)
    exit(1)
}
try png.write(to: URL(fileURLWithPath: CommandLine.arguments[1]))
