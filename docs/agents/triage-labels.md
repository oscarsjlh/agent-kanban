# Triage Labels

The skills speak in terms of five canonical triage roles. On the Kanban Board these roles map to columns or lifecycle commands instead of tracker labels.

| Canonical role | Kanban action | Meaning |
| --- | --- | --- |
| `needs-triage` | Issue is in `Inbox` | Maintainer needs to evaluate this Issue |
| `needs-info` | `kanban move <id> waiting --reason "needs info: ..."` | Waiting on reporter or external input |
| `ready-for-agent` | `kanban move <id> ready` | Fully specified, ready for an AFK agent |
| `ready-for-human` | `kanban move <id> waiting --reason "needs human: ..."` | Needs human implementation or decision |
| `wontfix` | `kanban move <id> wontfix` | Will not be actioned |

When a skill says to apply a label, perform the corresponding Kanban action instead.
