package quicklinks

// defaultLinks are the six links every user has, searched after the user's own links, as 2.0 lists them.
var defaultLinks = []Link{
	{Name: "GitHub", Keyword: "gh", URL: "https://github.com/search?q={query}"},
	{Name: "YouTube", Keyword: "yt", URL: "https://www.youtube.com/results?search_query={query}"},
	{Name: "Google Maps", Keyword: "maps", URL: "https://www.google.com/maps/search/{query}"},
	{Name: "Wikipedia", Keyword: "wiki", URL: "https://en.wikipedia.org/w/index.php?search={query}"},
	{Name: "npm", Keyword: "npm", URL: "https://www.npmjs.com/search?q={query}"},
	{Name: "Go Packages", Keyword: "gopkg", URL: "https://pkg.go.dev/search?q={query}"},
}
