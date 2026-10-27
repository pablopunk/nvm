export function macosAppFocusRestoreScript(
  quotedBundleId: string,
  pid?: number,
) {
  const processFilter = Number.isSafeInteger(pid)
    ? `unix id is ${pid} and bundle identifier is ${quotedBundleId}`
    : `bundle identifier is ${quotedBundleId}`;
  return `tell application "System Events"
set matchingProcesses to application processes whose ${processFilter}
if (count of matchingProcesses) is 0 then return false
set frontmost of item 1 of matchingProcesses to true
return true
end tell`;
}
