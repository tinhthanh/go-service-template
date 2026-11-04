---
name: trial
description: Measure how well this template guides an agent that has never seen it, by giving a fresh agent a product task in a worktree and scoring the result against the repository's own rules. Use when asked whether the template is ready for agents, after a batch of rule or skill changes, or to find what to improve next.
---

# Run a trial

1. **A worktree of main**: `git worktree add ../go-trial-<n> -b trial/<n> main`.
2. **A fresh agent** (no context from this session) in it, told only the task, as a person would
   write it: "Build tasks: the client's tasks, task, createTask, updateTask and deleteTask. Work as
   the repository says; you may not push: write the pull request into PR.md and commit." Don't
   point it at the skills: finding them is part of what is measured.
3. **Score it**, against the repository's rules, not taste:
   - `make verify` green; `make gates` with PR.md's title and body green;
   - the operations built answer as the client's mock does (refusals, defaults, roles, scope);
   - `add-feature`'s steps all there (queries in SQL, store in memory, servertest, store tests on
     Postgres, the role table in both places, a CHANGELOG line);
   - no rule bent: no `//nolint` without a reason, no check loosened, no `t.Skip`, no edit to
     generated code or the contract;
   - the `reviewer` agent (`.claude/agents/reviewer.md`) on its diff.
4. **Write it down** in `docs/trials.md`: the score, what it got wrong, and for each, what in the
   template would have prevented it (a check, a line in a skill, a better error message). Fix the
   template, not the trial's code. A subagent doesn't fire the repository's hooks: say so.
