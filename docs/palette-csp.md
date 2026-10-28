# Palette Content-Security-Policy

The palette document intentionally ships without a Content-Security-Policy for now.

The renderer has no HTML injection sink. React escapes text, `react-markdown` runs without raw HTML, and extension HTML only reaches the page inside sandboxed `srcDoc` iframes.

A CSP on the palette document is inherited by `srcDoc` webviews. Directives such as `form-action`, `base-uri`, and `script-src` would therefore break extension webviews. Dictation also needs `blob:` worklets, which a strict palette policy would block.

A useful CSP requires serving webview HTML from its own origin with its own policy. That is an architecture change and is tracked as future work.
