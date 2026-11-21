// Package tools is the set of intent-named tools the conversation offers the model: find and read files, list apps and
// open them, open a link, read or copy the clipboard, and move a file to the Trash. Each tool reaches the computer only
// through the platform ports it is given, and every path a tool names passes the guard first. A tool that changes or
// exposes something the user did not ask for for asks the user first (Confirm).
//
// Layer: ai. The tool names and their schemas are the model's contract; the copy the transcript shows is the Label and
// the one-line Summarize of each tool.
package tools
