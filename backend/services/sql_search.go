package services

import "strings"

// likeEscape is the escape character used by every `LIKE ... ESCAPE` clause in
// this package. A single backslash is understood by both PostgreSQL (the
// production driver) and SQLite (the test driver).
const likeEscape = `\`

// containsPattern builds a case-folded `%needle%` pattern for a LIKE search.
//
// The caller's text is escaped first: without it a bare `%` or `_` in the query
// is a wildcard, so a search for "%" would match every row instead of the
// literal percent sign. The pattern is always bound as a query parameter, never
// interpolated into the SQL text.
func containsPattern(query string) string {
	escaped := strings.NewReplacer(
		likeEscape, likeEscape+likeEscape,
		"%", likeEscape+"%",
		"_", likeEscape+"_",
	).Replace(strings.ToLower(strings.TrimSpace(query)))
	return "%" + escaped + "%"
}
