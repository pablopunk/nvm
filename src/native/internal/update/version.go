package update

import (
	"strconv"
	"strings"
)

// Compare orders two versions such as 2.0.0, 2.1.0-nightly.3 and 2.1.0 the way semantic versioning does: numbers
// numerically, and a prerelease before its release. It returns -1, 0 or 1.
func Compare(a, b string) int {
	aCore, aPre, _ := strings.Cut(strings.TrimPrefix(a, "v"), "-")
	bCore, bPre, _ := strings.Cut(strings.TrimPrefix(b, "v"), "-")
	if c := compareIdentifiers(aCore, bCore); c != 0 {
		return c
	}
	switch {
	case aPre == bPre:
		return 0
	case aPre == "":
		return 1
	case bPre == "":
		return -1
	}
	return compareIdentifiers(aPre, bPre)
}

// IsNewer reports whether candidate is a later version than current.
func IsNewer(candidate, current string) bool {
	return Compare(candidate, current) > 0
}

func compareIdentifiers(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		x, y := fieldAt(as, i), fieldAt(bs, i)
		if c := compareField(x, y); c != 0 {
			return c
		}
	}
	return 0
}

func fieldAt(fields []string, i int) string {
	if i < len(fields) {
		return fields[i]
	}
	return ""
}

func compareField(x, y string) int {
	xn, xerr := strconv.Atoi(x)
	yn, yerr := strconv.Atoi(y)
	if xerr == nil && yerr == nil {
		return sign(xn - yn)
	}
	return strings.Compare(x, y)
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}
