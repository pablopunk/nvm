package child

import "reflect"

// Symbols is the table of host packages extension source may import. api_symbols.go, generated from the api
// package, fills it in its init function. Regenerate it with go generate after any change to the api package.
var Symbols = map[string]map[string]reflect.Value{}

//go:generate sh -c "go run github.com/traefik/yaegi/cmd/yaegi extract -name child github.com/pablopunk/nvm/native/internal/ext/api && mv github_com-pablopunk-nvm-native-internal-ext-api.go api_symbols.go"
