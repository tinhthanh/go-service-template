# 0013: A day with no time is compared in UTC

**Status:** accepted, 2026-11-04 (from trial 2)

**Context.** Some fields are a day with no time (a task's `due`, `format: date`), and some answers
count against today (tasks' `late`). The contract sends no time zone, and the client's mock uses
the browser's day, which a server can't know.

**Decision.** A day is stored as a `date` and compared with the server's date in UTC: a task is
late when its due day is before today's UTC date and it isn't done. The day the client shows is
its own; only the count is the server's.

**Consequences.** Near midnight in a time zone far from UTC, a task may count as late an hour or
more before or after the person's own day ends.

**Revisit if.** The contract sends the person's time zone, or a workspace keeps one: compare in it.
