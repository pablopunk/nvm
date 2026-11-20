import Foundation

let args = CommandLine.arguments
switch args.count > 1 ? args[1] : "" {
case "calib":
    runCalibration(trials: Int(args[2]) ?? 60, out: args[3])
case "clip-save":
    let c = ClipboardSnapshot(); c.save(); try! c.save(toFile: args[2]); print("saved \(c.typeCount) typed items")
case "clip-restore":
    let c = ClipboardSnapshot(); try! c.load(fromFile: args[2]); c.restore(); print("restored \(c.typeCount) typed items")
case "explore":
    runExplore(configPath: args[2])
case "session":
    runSession(configPath: args[2])
default:
    FileHandle.standardError.write("usage: nvmbench calib <n> <out.csv> | session <config.json>\n".data(using: .utf8)!)
    exit(2)
}
