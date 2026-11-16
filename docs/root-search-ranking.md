# Root search ranking

A typed query ranks the root's rows by how well their name answers it, never by how much they are used. Usage only
decides between rows that answer the query equally well.

1. **Learned choice.** A row the user chose for this query, or for a shorter one this query extends, goes first, if its
   name still matches the query (tiers 1 to 4).
2. **Tier.** Exact (title, alias or keyword), title prefix, word start (or initials), substring, scattered letters
   starting at a word, then everything that matches only in a subtitle or in content. Query fallbacks (web search,
   Automate) follow all of them. A lower tier never outranks a higher one.
3. **Class, then use.** Inside a tier: applications, extension commands, built-in commands and settings, other rows. How
   often and how recently a row was used adds to its class, so daily use passes a class and one old use does not.
4. **Ties.** Shorter title, then alphabetical, then ID.

The empty query keeps its own order (1.x scores and usage). Views that order their own rows are not tiered.
The constants and the matching live in `src/native/internal/search` (`tiers.go`, `frecency.go`, `tiered.go`).
