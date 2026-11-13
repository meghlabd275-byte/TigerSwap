package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Webhook delivery states. A delivery is created pending when an event fires
// and only leaves pending once a dispatcher has claimed it.
const (
	WebhookDeliveryPending  = "pending"
	WebhookDeliverySending  = "sending"
	WebhookDeliverySent     = "sent"
	WebhookDeliveryFailed   = "failed"
)

// WebhookDelivery is one event delivery to one webhook endpoint. It is a durable
// queue row: a delivery is written in the same transaction as the event that
// caused it, then a dispatcher claims it, POSTs the signed payload and records
// the outcome. Attempts and NextAttemptAt give the retry its backoff schedule,
// so a restart resumes a delivery rather than losing it.
type WebhookDelivery struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	WebhookID  uuid.UUID `gorm:"type:uuid;index" json:"webhook_id"`
	UserID     string    `gorm:"index;size:100" json:"user_id"`
	Event      string    `gorm:"size:64;index" json:"event"`
	Payload    string    `gorm:"type:text" json:"payload"`
	Status     string    `gorm:"size:16;index;default:'pending'" json:"status"`
	Attempts   int       `gorm:"default:0" json:"attempts"`
	MaxAttempts int      `gorm:"default:5" json:"max_attempts"`
	// NextAttemptAt is when the dispatcher may next try. A row whose time has not
	// come is skipped, which is what spaces the retries out.
	NextAttemptAt time.Time  `gorm:"index" json:"next_attempt_at"`
	LastStatusCode int       `json:"last_status_code"`
	LastError      string    `gorm:"size:255" json:"last_error"`
	DeliveredAt    *time.Time `json:"delivered_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (d *WebhookDelivery) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}
