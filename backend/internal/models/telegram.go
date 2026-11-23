package models

import "time"

// ChannelStats is the latest known state of the public Telegram channel.
type ChannelStats struct {
	Subscribers int
	UpdatedAt   time.Time
}
