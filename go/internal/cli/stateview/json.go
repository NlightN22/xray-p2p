package stateview

import "time"

// JSONResult is the typed machine-readable heartbeat state view.
type JSONResult struct {
	Tunnels []JSONTunnel `json:"tunnels"`
}

type JSONTunnel struct {
	Tag                 string       `json:"tag"`
	Host                string       `json:"host"`
	User                string       `json:"user"`
	ClientIP            string       `json:"client_ip"`
	Alive               bool         `json:"alive"`
	Status              string       `json:"status"`
	Reason              string       `json:"reason"`
	AgeMillis           int64        `json:"age_millis"`
	LastRTTMillis       int64        `json:"last_rtt_millis"`
	MinRTTMillis        int64        `json:"min_rtt_millis"`
	MaxRTTMillis        int64        `json:"max_rtt_millis"`
	AverageRTTMillis    float64      `json:"average_rtt_millis"`
	Samples             int64        `json:"samples"`
	LastSeen            *string      `json:"last_seen"`
	Healthy             *bool        `json:"healthy"`
	Mode                string       `json:"mode"`
	Capability          string       `json:"capability"`
	LastSuccess         *string      `json:"last_success"`
	FailureStage        string       `json:"failure_stage"`
	Failure             string       `json:"failure"`
	ConsecutiveFailures int          `json:"consecutive_failures"`
	EndpointID          string       `json:"endpoint_id"`
	Attempts            int64        `json:"attempts"`
	Traffic             *JSONTraffic `json:"traffic"`
}

type JSONTraffic struct {
	UploadBytes   uint64 `json:"upload_bytes"`
	DownloadBytes uint64 `json:"download_bytes"`
	TotalBytes    uint64 `json:"total_bytes"`
}

// BuildJSONResult converts the shared read model without parsing rendered text.
func BuildJSONResult(view SnapshotView) JSONResult {
	result := JSONResult{Tunnels: make([]JSONTunnel, 0, len(view.Snapshots))}
	for _, snapshot := range view.Snapshots {
		entry := snapshot.Entry
		item := JSONTunnel{
			Tag: entry.Tag, Host: entry.Host, User: entry.User, ClientIP: entry.ClientIP,
			Alive: snapshot.Alive, Status: string(entry.Status), Reason: snapshot.Reason,
			AgeMillis: snapshot.Age.Milliseconds(), LastRTTMillis: entry.LastRTTMillis,
			MinRTTMillis: entry.MinRTTMillis, MaxRTTMillis: entry.MaxRTTMillis,
			AverageRTTMillis: snapshot.AvgRTTMillis, Samples: entry.Samples,
			LastSeen: optionalRFC3339(entry.LastSeen), Healthy: entry.Healthy,
			Mode: string(entry.Mode), Capability: string(entry.Capability),
			LastSuccess: optionalRFC3339(entry.LastSuccess), FailureStage: string(entry.FailureStage),
			Failure: entry.Failure, ConsecutiveFailures: entry.ConsecutiveFailures,
			EndpointID: entry.EndpointID, Attempts: entry.Attempts,
		}
		if traffic, ok := view.Stats[statsKey(entry.User)]; ok && traffic.Available {
			item.Traffic = &JSONTraffic{
				UploadBytes: traffic.UploadBytes, DownloadBytes: traffic.DownloadBytes, TotalBytes: traffic.TotalBytes,
			}
		}
		result.Tunnels = append(result.Tunnels, item)
	}
	return result
}

func optionalRFC3339(value time.Time) *string {
	if value.IsZero() {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}
