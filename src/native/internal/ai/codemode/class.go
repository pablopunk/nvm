package codemode

// Class says what a service call can do, and with it whether the user must allow it. The class of every (service,
// operation) pair is fixed in Go, in the rule table next to the gate; the program and the model cannot declare or change
// one, and a pair with no rule is refused.
type Class string

const (
	// ClassRead reads data the user already has in Nevermind. It runs and is journaled.
	ClassRead Class = "read"
	// ClassWrite adds or changes data and is easily undone or additive. It runs without asking, and the transcript
	// shows a row for it.
	ClassWrite Class = "write"
	// ClassSensitiveRead reads data that may hold secrets, such as the clipboard history. The user is asked, with what
	// is read, before any of it reaches the model; an allow covers repeats of the exact operation and resource in the run.
	ClassSensitiveRead Class = "sensitive-read"
	// ClassDestructive removes data. The user is asked every time, with a description the gate builds from host data.
	ClassDestructive Class = "destructive"
	// ClassExternal runs something outside Nevermind's own data, such as a command, or sends data out of it, such as a
	// clipboard write. The user is asked every time.
	ClassExternal Class = "external"
	// ClassSetting changes how Nevermind behaves everywhere, such as a global shortcut. The user is asked every time,
	// with the change described from host data.
	ClassSetting Class = "setting"
)

// asksEachTime reports whether every call of the class waits for the user's answer.
func (c Class) asksEachTime() bool {
	return c == ClassDestructive || c == ClassExternal || c == ClassSetting
}

// allowCoversRepeats reports whether the user's allow of a call of the class also covers the exact same call later in
// the run.
func (c Class) allowCoversRepeats() bool {
	return c == ClassSensitiveRead
}

// asks reports whether a call of the class waits for the user's answer, the first time or every time.
func (c Class) asks() bool {
	return c.asksEachTime() || c.allowCoversRepeats()
}

// changesData reports whether a call of the class can change something, which the model is told about.
func (c Class) changesData() bool {
	return c == ClassWrite || c == ClassDestructive || c == ClassExternal || c == ClassSetting
}
