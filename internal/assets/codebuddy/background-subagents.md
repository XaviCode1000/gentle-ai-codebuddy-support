<!-- gentle-ai:codebuddy-background-subagents -->
### CodeBuddy Background Subagent Policy

Use the Agent tool with `run_in_background: true` only for independent, read-only exploration, audit, or review work where the parent can continue non-overlapping work.

At the parent level, allow no more than 2 concurrent background tasks. Completion notifications only: do not poll, sleep, run status checks, or proactively read for completion. Call `TaskOutput` when the result is actually needed and `TaskStop` to cancel.

Use foreground tasks when the result is needed before the next action, for user decisions, writers, dependent verify evidence, archive, formal RDD/4R lenses, refuters, fix validators, Judgment Day actors, or dependent phases.

Do not duplicate launches or work, and do not overlap files or topics. Never run parallel writers in one worktree.

Background tasks are process-local and non-durable. A restart loses them; make no recovery claim. If `run_in_background` is absent from the Agent tool schema, or background tasks are disabled (`CODEBUDDY_CODE_DISABLE_BACKGROUND_TASKS`), omit it and run the task in the foreground.
<!-- /gentle-ai:codebuddy-background-subagents -->
