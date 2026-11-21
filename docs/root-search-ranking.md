# Root search ranking

A typed query ranks the root's rows by how well their name answers it, never by how much they are used. Usage only
decides between rows that answer the query equally well.

1. **Tier.** Exact (title, alias or keyword), title prefix, word start (or initials: "gc" for Google Chrome), substring,
   then everything that matches only in a subtitle, a keyword, a path or content. Letters scattered through a name
   never match: "loom" is not "Log in to Nevermind" or "Adobe Lightroom". Query fallbacks (web search, Automate) follow
   all of them. A lower tier never outranks a higher one, for any reason: not usage, not a learned choice, and not the
   order rows arrive in while providers answer.
2. **Learned choice.** Inside a tier, a row the user chose for this query, or for a shorter one this query extends, goes
   first, if its name matches the query (tiers 1 to 4). It moves a row to the top of its own tier and no further.
3. **Class, then use.** Inside a tier: applications, extension commands, built-in commands and settings, other rows. How
   often and how recently a row was used adds to its class, so daily use passes a class and one old use does not.
4. **Ties.** Shorter title, then alphabetical, then ID.

A file row at the root matches by its file name (tiers 1 to 4). The file index also answers names that only hold the
query's letters scattered or in any order; the root drops them. Meaning search stays in Find Files by Meaning. A row that
matches by content, or only by path, is in the content tier.

While providers answer, rows already on screen keep their order inside their tier; a row that arrives late still goes
above every worse tier.

The empty query keeps its own order (1.x scores and usage). Views that order their own rows are not tiered.
The constants and the matching live in `src/native/internal/search` (`tiers.go`, `frecency.go`, `tiered.go`).
