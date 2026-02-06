# Task Management

Track tasks, commitments, and follow-ups using a simple TASKS.md file. Helps you stay on top of what needs doing, what you are waiting on, and what is done.

## Usage

Ask me about your tasks, add new ones, mark things done, or review what is on your plate.

## File Location

Tasks are tracked in `TASKS.md` in the current working directory. If it does not exist, create it with this template:

```markdown
# Tasks

## Active

## Waiting On

## Someday

## Done
```

## Task Format

- `- [ ] **Task title** - context, for whom, due date`
- Sub-bullets for additional details
- Completed: `- [x] ~~Task~~ (date)`

## How to Interact

**When user asks "what's on my plate" / "my tasks":**
- Read TASKS.md
- Summarize Active and Waiting On sections
- Highlight anything overdue or urgent

**When user says "add a task" / "remind me to":**
- Add to Active section with `- [ ] **Task**` format
- Include context if provided (who it is for, due date)

**When user says "done with X" / "finished X":**
- Find the task
- Change `[ ]` to `[x]`, add strikethrough and completion date
- Move to Done section

**When user asks "what am I waiting on":**
- Read the Waiting On section
- Note how long each item has been waiting

## Extracting Tasks from Conversations

When summarizing meetings or conversations, offer to add extracted tasks:
- Commitments the user made ("I'll send that over")
- Action items assigned to them
- Follow-ups mentioned

Ask before adding -- do not auto-add without confirmation.

## Conventions

- **Bold** the task title for scannability
- Include "for [person]" when it is a commitment to someone
- Include "due [date]" for deadlines
- Include "since [date]" for waiting items
- Keep Done section for about one week, then clear old items

## Example Prompts

- "What's on my plate?"
- "Add a task: review PR for the auth module"
- "I'm done with the quarterly report"
- "What am I waiting on?"
- "Extract action items from these meeting notes"

## Tools

- `read_file`: Read TASKS.md to check current tasks
- `write_file`: Create TASKS.md if it does not exist
- `edit_file`: Add, complete, or move tasks
