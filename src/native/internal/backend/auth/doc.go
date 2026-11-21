// Package auth is the account side of the backend client: device sign-in and deep-link sign-in, the bearer
// token in the platform keychain, sign-out, the backend environment switch, the bring-your-own provider key and
// the spend-cap notices.
//
// Layer: backend. It imports the backend client, the platform ports, the store and logx. Tokens and keys live in
// the keychain only; the non-secret profile (environment, account identity, BYO metadata) lives in one store
// document. Nothing here reads the wall clock or logs a token, an email or a key.
package auth
