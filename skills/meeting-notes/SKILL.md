# Meeting Notes

Process meeting transcripts or rough notes into structured summaries with decisions, action items, and follow-ups.

## Usage

Share meeting notes, a transcript, or a recording summary, and I will organize it into a structured format with key decisions and action items extracted.

## Capabilities

- Structure raw notes into organized meeting summaries
- Extract decisions, action items, and open questions
- Identify owners and deadlines for action items
- Highlight items that need follow-up
- Track recurring meeting topics over time

## Meeting Summary Format

```markdown
# Meeting: [Title]
**Date:** [Date]
**Attendees:** [Names]
**Duration:** [Length]

## Summary
[2-3 sentence overview of what was discussed and decided]

## Decisions
- [Decision 1 -- who decided, rationale if noted]
- [Decision 2]

## Action Items
- [ ] **[Owner]**: [Task description] (due: [date])
- [ ] **[Owner]**: [Task description] (due: [date])

## Discussion Notes
### [Topic 1]
- [Key points discussed]
- [Different viewpoints if any]

### [Topic 2]
- [Key points discussed]

## Open Questions
- [Question that was raised but not resolved -- who should answer]

## Follow-ups for Next Meeting
- [Items to revisit]
```

## Processing Guidelines

### Extracting Decisions
Look for signals like:
- "We decided to..."
- "Let's go with..."
- "The plan is..."
- "We agreed that..."
- Consensus statements after discussion

### Extracting Action Items
Look for signals like:
- "[Person] will..."
- "Can you [do something] by [date]?"
- "I'll take care of..."
- "Next step is..."
- "TODO:" or "Action:"

For each action item, capture:
- **Who** is responsible
- **What** they need to do (specific and actionable)
- **When** it is due (if mentioned)

### Handling Ambiguity
- If the owner is unclear, note it: "**TBD**: [task] (needs owner)"
- If the deadline is unclear, omit it rather than guess
- If a decision seems tentative, note it: "[Decision] (tentative, pending [condition])"

## After Processing

Offer to:
- Add action items to TASKS.md (if task-management skill is available)
- Send a summary to attendees
- Create follow-up reminders

## Example Prompts

- "Here are my notes from today's standup, organize them"
- "Process this meeting transcript into a summary"
- "Extract the action items from these notes"
- "What decisions were made in this meeting?"

## Tools

- `read_file`: Read meeting notes or transcripts
- `write_file`: Save structured meeting summaries
- `edit_file`: Update existing meeting notes
- `list_dir`: Find meeting note files
