package github

// ActionEvents is the list of all GitHub Actions-related webhook event types.
// https://docs.github.com/en/webhooks/webhook-events-and-payloads
var ActionEvents = []string{
	"workflow_run",
	"workflow_job",
	"workflow_dispatch",
	"check_run",
	"check_suite",
	"deployment",
	"deployment_status",
	"push",
	"pull_request",
	"pull_request_review",
	"pull_request_review_comment",
	"create",
	"delete",
	"release",
	"status",
	"repository_dispatch",
	"ping",
}

// Event is the envelope forwarded to WebSocket clients.
type Event struct {
	EventType string `json:"event_type"`
	Delivery  string `json:"delivery"`
	// Payload holds the raw GitHub JSON payload so clients get the full picture.
	Payload any `json:"payload"`
}
