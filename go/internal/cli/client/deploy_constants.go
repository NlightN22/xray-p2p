package clientcmd

import "time"

const (
	deployLinkTTL                 = 10 * time.Minute
	socksReadyTimeout             = 30 * time.Second
	socksProbeInterval            = 500 * time.Millisecond
	deployCompletionNotifyTimeout = 30 * time.Second
	socksPingTimeout              = 45 * time.Second
	applyRequestTimeout           = 45 * time.Second
)
