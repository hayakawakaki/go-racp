package transport

const (
	noticeLookReset      = "look_reset"
	noticeLocationReset  = "location_reset"
	noticeOnlineLocked   = "online_locked"
	noticeCooldownActive = "cooldown_active"
	noticeNotFound       = "not_found"
)

var noticeText = map[string]string{
	noticeLookReset:      "Look reset.",
	noticeLocationReset:  "Location reset.",
	noticeOnlineLocked:   "Log out the character before changing it.",
	noticeCooldownActive: "Cooldown still active. Try again later.",
	noticeNotFound:       "Character not found.",
}
