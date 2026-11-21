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
	// ClassSensitiveRead reads data that may hold secrets, such as the clipboard history. The user is asked once per
	// run and service, with what is read, before any of it reaches the model.
	ClassSensitiveRead Class = "sensitive-read"
	// ClassDestructive removes data. The user is asked every time, with a description the gate builds from host data.
	ClassDestructive Class = "destructive"
	// ClassExternal runs something outside Nevermind's own data, such as a command. The user is asked every time.
	ClassExternal Class = "external"
)

// asksEachTime reports whether every call of the class waits for the user's answer.
func (c Class) asksEachTime() bool {
	return c == ClassDestructive || c == ClassExternal
}

// asksOncePerService reports whether the first call of the class on a service waits for the user's answer.
func (c Class) asksOncePerService() bool {
	return c == ClassSensitiveRead
}

// changesData reports whether a call of the class can change something, which the model is told about.
func (c Class) changesData() bool {
	return c == ClassWrite || c == ClassDestructive || c == ClassExternal
}
