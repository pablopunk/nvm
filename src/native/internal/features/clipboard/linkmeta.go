package clipboard

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// LinkMetadata is what a page declares about itself. Every field is optional. ImageURL is absolute and may
// point anywhere; the service fetches it through the host, never the renderer.
type LinkMetadata struct {
	Title       string
	Description string
	SiteName    string
	ImageURL    string
}

var (
	// attributePattern matches one attribute at the start of its input: name, then an optional quoted or
	// bare value. It is applied at each position of a tag, so a ">" inside quotes stays text.
	attributePattern = regexp.MustCompile("^([^\\s\"'<>/=]+)(?:\\s*=\\s*(?:\"([^\"]*)\"|'([^']*)'|([^\\s\"'=<>`]+)))?")
	metaOpenPattern  = regexp.MustCompile(`(?i)<meta`)
	titlePattern     = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	decimalEntity    = regexp.MustCompile(`&#(\d+);`)
	hexEntity        = regexp.MustCompile(`(?i)&#x([0-9a-f]+);`)
	namedEntities    = strings.NewReplacer("&quot;", `"`, "&apos;", "'", "&#39;", "'", "&lt;", "<", "&gt;", ">", "&amp;", "&")
)

// ParseLinkMetadata reads Open Graph, Twitter and plain HTML metadata from a page. Open Graph wins over Twitter,
// Twitter over the plain description; the title falls back to the document title. Relative image addresses
// resolve against pageURL.
func ParseLinkMetadata(html, pageURL string) LinkMetadata {
	tags := metaTags(html)
	pick := func(keys ...string) string {
		for _, key := range keys {
			if value := tags[key]; value != "" {
				return value
			}
		}
		return ""
	}
	title := pick("og:title", "twitter:title")
	if title == "" {
		if match := titlePattern.FindStringSubmatch(html); match != nil {
			title = decodeEntities(match[1])
		}
	}
	return LinkMetadata{
		Title:       title,
		Description: pick("og:description", "twitter:description", "description"),
		SiteName:    pick("og:site_name"),
		ImageURL:    absoluteHTTPURL(pick("og:image", "og:image:url", "twitter:image", "twitter:image:src"), pageURL),
	}
}

// metaTags maps each meta tag's property (or name) to its content, first occurrence wins, keys lowercased.
func metaTags(html string) map[string]string {
	tags := map[string]string{}
	for _, loc := range metaOpenPattern.FindAllStringIndex(html, -1) {
		next := loc[1]
		if next >= len(html) || !isTagBoundary(html[next]) {
			continue
		}
		attributes := readAttributes(html, next)
		key, ok := attributes["property"]
		if !ok {
			key = attributes["name"]
		}
		content, hasContent := attributes["content"]
		if key == "" || !hasContent {
			continue
		}
		lower := strings.ToLower(key)
		if _, seen := tags[lower]; !seen {
			tags[lower] = decodeEntities(content)
		}
	}
	return tags
}

// isTagBoundary reports whether a character may follow "<meta": whitespace, a slash or the end of the tag.
func isTagBoundary(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '/' || c == '>'
}

// readAttributes reads one tag's attributes from start, treating ">" inside quoted values as text. The first
// value of a repeated name wins.
func readAttributes(html string, start int) map[string]string {
	attributes := map[string]string{}
	index := start
	for index < len(html) {
		for index < len(html) && isSpaceOrSlash(html[index]) {
			index++
		}
		if index >= len(html) || html[index] == '>' {
			break
		}
		loc := attributePattern.FindStringSubmatchIndex(html[index:])
		if loc == nil {
			index++
			continue
		}
		name := strings.ToLower(html[index+loc[2] : index+loc[3]])
		if _, seen := attributes[name]; !seen {
			attributes[name] = attributeValue(html, index, loc)
		}
		index += loc[1]
	}
	return attributes
}

// attributeValue returns the quoted or bare value of a matched attribute, or "" when it has none.
func attributeValue(html string, index int, loc []int) string {
	for group := 4; group <= 8; group += 2 {
		if loc[group] >= 0 {
			return html[index+loc[group] : index+loc[group+1]]
		}
	}
	return ""
}

// isSpaceOrSlash reports whether a byte separates attributes.
func isSpaceOrSlash(c byte) bool {
	return isTagBoundary(c) && c != '>'
}

// decodeEntities decodes numeric and named entities and collapses whitespace.
func decodeEntities(value string) string {
	value = decimalEntity.ReplaceAllStringFunc(value, func(match string) string {
		code, err := strconv.Atoi(match[2 : len(match)-1])
		if err != nil || code > 0x10FFFF {
			return match
		}
		return string(rune(code))
	})
	value = hexEntity.ReplaceAllStringFunc(value, func(match string) string {
		code, err := strconv.ParseInt(match[3:len(match)-1], 16, 32)
		if err != nil || code > 0x10FFFF {
			return match
		}
		return string(rune(code))
	})
	value = namedEntities.Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

// absoluteHTTPURL resolves value against base and returns it when it is an http or https address.
func absoluteHTTPURL(value, base string) string {
	if value == "" {
		return ""
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return ""
	}
	ref, err := url.Parse(value)
	if err != nil {
		return ""
	}
	resolved := baseURL.ResolveReference(ref)
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return ""
	}
	return resolved.String()
}
