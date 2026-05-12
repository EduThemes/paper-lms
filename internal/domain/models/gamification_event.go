package models

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/datatypes"
)

// GamificationEvent is the xAPI-shaped event store row. Every
// gamification-relevant action in Paper LMS emits one row via
// service/gamification.Emit. The rules engine subscribes to this stream
// and produces RuleEvaluation + WalletTransaction rows downstream.
//
// IDs are uint to match the rest of the Paper LMS schema; the xAPI export
// layer synthesizes IRIs at serialization time. Source values:
// "internal" | "lti" | "webhook" | "migration_import". The
// (Source, SourceEventID) pair is uniquely indexed (000032) so
// re-deliveries from upstream systems are absorbed.
//
// All indexes live in the SQL migration chain (000032). The model does
// not duplicate them via GORM tags — AutoMigrate would otherwise create
// shadow indexes (no DESC, no partial WHERE) that the parity test would
// flag as drift.
type GamificationEvent struct {
	ID            uint           `json:"id" gorm:"primaryKey"`
	OccurredAt    time.Time      `json:"occurred_at" gorm:"not null"`
	EmittedAt     time.Time      `json:"emitted_at" gorm:"not null;default:now()"`
	TenantID      uint           `json:"tenant_id" gorm:"not null"`
	ActorID       uint           `json:"actor_id" gorm:"not null"`
	Verb          string         `json:"verb" gorm:"not null"`
	ObjectType    string         `json:"object_type" gorm:"not null"`
	ObjectID      *uint          `json:"object_id,omitempty"`
	Result        datatypes.JSON `json:"result,omitempty" gorm:"type:jsonb"`
	Context       datatypes.JSON `json:"context,omitempty" gorm:"type:jsonb"`
	Source        string         `json:"source" gorm:"not null;default:'internal'"`
	SourceEventID *string        `json:"source_event_id,omitempty"`
	PolicyFlags   pq.StringArray `json:"policy_flags" gorm:"type:text[];not null;default:'{}'"`
	Signature     *string        `json:"signature,omitempty"`
}

// TableName pins the table name so the parity-test contract is explicit
// and resilient to future GORM pluralization changes.
func (GamificationEvent) TableName() string { return "gamification_events" }
