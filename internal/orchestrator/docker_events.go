package orchestrator

import (
	"context"
	"time"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
)

// ContainerEvent represents a relevant Docker container lifecycle event
type ContainerEvent struct {
	Action      string // "die", "start"
	ContainerID string
}

// WatchEvents streams Docker container events filtered to OpenVeth containers.
// It calls the provided callback for each relevant event until ctx is cancelled.
func (m *Manager) WatchEvents(ctx context.Context, callback func(ContainerEvent)) {
	f := filters.NewArgs()
	f.Add("type", "container")
	f.Add("event", "die")
	f.Add("event", "start")
	f.Add("label", "openveth=true")

	eventsCh, errCh := m.cli.Events(ctx, events.ListOptions{Filters: f})

	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-errCh:
			if !ok {
				return
			}
			if err != nil && ctx.Err() == nil {
				m.logger.Warn("docker event stream error, restarting", "error", err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(3 * time.Second):
				}
				eventsCh, errCh = m.cli.Events(ctx, events.ListOptions{Filters: f})
			}
		case ev := <-eventsCh:
			callback(ContainerEvent{
				Action:      string(ev.Action),
				ContainerID: ev.Actor.ID,
			})
		}
	}
}
