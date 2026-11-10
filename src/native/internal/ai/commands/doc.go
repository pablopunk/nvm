// Package commands holds the logic of Fix Selected Text with AI: it corrects a text piece by piece with the fast model
// and guards each piece against changes the correction must not make. The built-in AI commands run it through the
// extension API, and Triage, which needed no shared logic, lives in that built-in.
package commands
