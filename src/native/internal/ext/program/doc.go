// Package program is the sandbox contract of the programs an AI conversation writes. A program is the body of one Go
// function, run by the extension interpreter in a throwaway child. The model never writes imports: the host reads the
// body, refuses the constructs a program must not use, and generates the import list from a fixed allowlist. The child
// then loads only the symbols of those packages and of the extension api, so a program cannot name the operating
// system, the network, a process or unsafe memory even if a check here missed it.
//
// Yaegi is not a security boundary. The boundary is the gate in front of the host services (package codemode); this
// package keeps the program from reaching anything the gate does not see.
//
// Layer: ext. The package imports the standard library only.
package program
