package client

import "github.com/NlightN22/xray-p2p/go/internal/heartbeat"

func completeHeartbeatCheck(capability heartbeat.Capability, report func() error) error {
	if capability == heartbeat.CapabilityXP2PDiag {
		return nil
	}
	return report()
}
