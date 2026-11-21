package actioncli

import "strings"

// Invocation is the command line that runs actionID, quoted for goos's usual shell, to copy from the palette or write
// into a compositor binding that runs it through a shell.
func Invocation(program, actionID, goos string) string {
	quote := PosixQuote
	if goos == "windows" {
		quote = windowsQuote
	}
	return quote(program) + " " + RunVerb + " " + quote(actionID)
}

// PosixQuote quotes s as one word for a POSIX shell. Words made only of safe characters stay as they are.
func PosixQuote(s string) string {
	if s != "" && strings.Trim(s, posixSafe) == "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

const posixSafe = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_./:=@+,%"

// windowsQuote quotes s as one argument for the Windows argument parser.
func windowsQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\"") {
		return s
	}
	var b strings.Builder
	b.WriteByte('"')
	backslashes := 0
	for _, r := range s {
		switch r {
		case '\\':
			backslashes++
			continue
		case '"':
			b.WriteString(strings.Repeat(`\`, backslashes*2+1))
		default:
			b.WriteString(strings.Repeat(`\`, backslashes))
		}
		backslashes = 0
		b.WriteRune(r)
	}
	b.WriteString(strings.Repeat(`\`, backslashes*2))
	b.WriteByte('"')
	return b.String()
}
