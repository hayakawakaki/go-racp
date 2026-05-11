package mailer

import (
	"fmt"

	"github.com/wneessen/go-mail"
)

func NewClient(host string, port int, requireTLS bool) (*mail.Client, error) {
	policy := mail.TLSOpportunistic
	if requireTLS {
		policy = mail.TLSMandatory
	}
	c, err := mail.NewClient(host,
		mail.WithPort(port),
		mail.WithTLSPolicy(policy),
	)
	if err != nil {
		return nil, fmt.Errorf("mailer: new client: %w", err)
	}
	return c, nil
}
