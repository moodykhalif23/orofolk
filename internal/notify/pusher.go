package notify

import (
	"context"
	"errors"

	pusher "github.com/pusher/pusher-http-go/v5"
)

// NoopPublisher is the default when Pusher is not configured. Publishing is a
// silent success (notifications still persist and are served over HTTP), and
// channel auth is reported as unavailable.
type NoopPublisher struct{}

func (NoopPublisher) Publish(context.Context, []string, string, any) error { return nil }
func (NoopPublisher) Enabled() bool                                        { return false }
func (NoopPublisher) Authorize([]byte) ([]byte, error) {
	return nil, errors.New("realtime notifications not configured")
}

// PusherPublisher delivers notifications over Pusher Channels and signs private
// channel subscriptions. It implements both Publisher and Authorizer.
type PusherPublisher struct {
	client  *pusher.Client
	key     string
	cluster string
}

// NewPusherPublisher builds a Pusher-backed publisher. Returns (nil, false) when
// the required credentials are absent, so the caller falls back to NoopPublisher.
func NewPusherPublisher(appID, key, secret, cluster string) (*PusherPublisher, bool) {
	if appID == "" || key == "" || secret == "" || cluster == "" {
		return nil, false
	}
	return &PusherPublisher{
		client: &pusher.Client{
			AppID:   appID,
			Key:     key,
			Secret:  secret,
			Cluster: cluster,
			Secure:  true,
		},
		key:     key,
		cluster: cluster,
	}, true
}

// Publish triggers event on the given channels. Pusher caps a single trigger at
// 100 channels; notification fan-out is one channel per call, well under that.
func (p *PusherPublisher) Publish(_ context.Context, channels []string, event string, data any) error {
	if len(channels) == 0 {
		return nil
	}
	return p.client.TriggerMulti(channels, event, data)
}

// Enabled reports that real-time is available.
func (p *PusherPublisher) Enabled() bool { return true }

// Authorize signs the raw Pusher private-channel auth params. The caller must
// have already validated that the requested channel belongs to the user.
func (p *PusherPublisher) Authorize(params []byte) ([]byte, error) {
	return p.client.AuthorizePrivateChannel(params)
}

// Key returns the public app key (safe to expose to the browser).
func (p *PusherPublisher) Key() string { return p.key }

// Cluster returns the Pusher cluster (safe to expose to the browser).
func (p *PusherPublisher) Cluster() string { return p.cluster }
