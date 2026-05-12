package accountchange

type Type uint8

const (
	Unknown Type = iota
	Password
	Email
)
