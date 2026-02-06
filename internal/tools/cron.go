package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/hkuds/ubot/internal/cron"
)

// CronTool allows the LLM to manage proactive scheduled reminders.
type CronTool struct {
	BaseTool
	scheduler *cron.Scheduler
}

// NewCronTool creates a new CronTool backed by the given Scheduler.
func NewCronTool(scheduler *cron.Scheduler) *CronTool {
	return &CronTool{
		BaseTool: NewBaseTool(
			"cron",
			"Manage proactive scheduled reminders. Use 'add' to create a new recurring reminder with a cron schedule or interval (e.g. '@every 5m', '0 9 * * 1-5'). Use 'remove' to delete a reminder by ID. Use 'list' to see all active reminders.",
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"add", "remove", "list"},
						"description": "The action to perform: add, remove, or list.",
					},
					"schedule": map[string]interface{}{
						"type":        "string",
						"description": "Cron expression (e.g. '*/5 * * * *', '0 9 * * 1-5') or interval (e.g. '@every 5m', '@every 1h'). Required for 'add'.",
					},
					"instruction": map[string]interface{}{
						"type":        "string",
						"description": "What the reminder should do when it fires. This becomes the LLM prompt. Required for 'add'.",
					},
					"channel": map[string]interface{}{
						"type":        "string",
						"description": "The channel to send the reminder to (e.g. 'telegram', 'cli'). Required for 'add'.",
					},
					"chat_id": map[string]interface{}{
						"type":        "string",
						"description": "The chat/conversation ID to send the reminder to. Required for 'add'.",
					},
					"job_id": map[string]interface{}{
						"type":        "string",
						"description": "The job ID to remove. Required for 'remove'.",
					},
				},
				"required": []string{"action"},
			},
		),
		scheduler: scheduler,
	}
}

// Execute runs the cron tool action.
func (t *CronTool) Execute(ctx context.Context, params map[string]interface{}) (string, error) {
	action, err := GetStringParam(params, "action")
	if err != nil {
		return "", fmt.Errorf("cron: %w", err)
	}

	switch action {
	case "add":
		return t.add(params)
	case "remove":
		return t.remove(params)
	case "list":
		return t.list()
	default:
		return "", fmt.Errorf("cron: unknown action %q (use add, remove, or list)", action)
	}
}

func (t *CronTool) add(params map[string]interface{}) (string, error) {
	schedule, err := GetStringParam(params, "schedule")
	if err != nil {
		return "", fmt.Errorf("cron add: %w", err)
	}
	instruction, err := GetStringParam(params, "instruction")
	if err != nil {
		return "", fmt.Errorf("cron add: %w", err)
	}
	channel, err := GetStringParam(params, "channel")
	if err != nil {
		return "", fmt.Errorf("cron add: %w", err)
	}
	chatID, err := GetStringParam(params, "chat_id")
	if err != nil {
		return "", fmt.Errorf("cron add: %w", err)
	}

	id, err := t.scheduler.AddJob(schedule, instruction, channel, chatID)
	if err != nil {
		return "", fmt.Errorf("cron add: %w", err)
	}

	return fmt.Sprintf("Reminder added (ID: %s). Schedule: %s", id, schedule), nil
}

func (t *CronTool) remove(params map[string]interface{}) (string, error) {
	jobID, err := GetStringParam(params, "job_id")
	if err != nil {
		return "", fmt.Errorf("cron remove: %w", err)
	}

	if err := t.scheduler.RemoveJob(jobID); err != nil {
		return "", fmt.Errorf("cron remove: %w", err)
	}

	return fmt.Sprintf("Reminder %s removed.", jobID), nil
}

func (t *CronTool) list() (string, error) {
	jobs := t.scheduler.ListJobs()
	if len(jobs) == 0 {
		return "No active reminders.", nil
	}

	var sb strings.Builder
	sb.WriteString("Active reminders:\n\n")
	for _, j := range jobs {
		sb.WriteString(fmt.Sprintf("- ID: %s | Schedule: %s | Channel: %s | Chat: %s\n  Instruction: %s\n",
			j.ID, j.Schedule, j.Channel, j.ChatID, j.Instruction))
	}
	return sb.String(), nil
}
