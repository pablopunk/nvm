package api

import "slices"

// lucideMenuBarIcons names the Lucide icons (ISC) embedded in internal/menubar/icons, each rendered black and white at 32
// pixels. It includes the brand logos, such as github, that Lucide 1 removed.
var lucideMenuBarIcons = slices.Concat(
	lucideMenuBarIconsAB,
	lucideMenuBarIconsC,
	lucideMenuBarIconsDEF,
	lucideMenuBarIconsGHIJKL,
	lucideMenuBarIconsMNOPQ,
	lucideMenuBarIconsRS,
	lucideMenuBarIconsTUVWXYZ,
)
