package app

import "errors"

var ErrEmailChangeCooldown = errors.New("account: email-change cooldown active")
