// Package stubborn runs workflows that survive crashes.
//
// A workflow is an ordinary Go function. Stubborn records the result of
// every step in Postgres or SQLite, so when the process dies, any worker can
// pick the workflow up and continue after the last step that completed.
// There is no server: the database is the only shared component.
package stubborn
