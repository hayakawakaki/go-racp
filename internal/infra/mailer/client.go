package mailer

import (
	"fmt"

	"github.com/wneessen/go-mail"
)

func NewClient(host string, port int) (*mail.Client, error) {
	c, err := mail.NewClient(host, mail.WithPort(port))
	if err != nil {
		return nil, fmt.Errorf("mailer: new client: %w", err)
	}
	return c, nil
}
