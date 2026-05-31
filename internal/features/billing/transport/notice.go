package transport

const (
	noticeSuccess     = "success"
	noticeCancel      = "cancel"
	noticeInvalid     = "invalid"
	noticeUnavailable = "unavailable"
)

var storeNoticeText = map[string]string{
	noticeUnavailable: "The store is currently unavailable. Please try again later.",
	noticeInvalid:     "That request could not be processed. Please try again.",
}

func noticeMessage(code string) string {
	return storeNoticeText[code]
}
