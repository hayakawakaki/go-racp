package domain

import "time"

type AuditKind string

const (
	AuditBan     AuditKind = "ban"
	AuditUnban   AuditKind = "unban"
	AuditSetRole AuditKind = "set_role"
)

type AuditEntry struct {
	At           time.Time
	Kind         AuditKind
	Reason       string
	BeforeValue  string
	AfterValue   string
	ID           int64
	ActorUserID  int
	TargetUserID int
}
