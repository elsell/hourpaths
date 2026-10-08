package identity

// Provider names come from authenticated broker claims, never email or routing.
type Provider string

const (
	ProviderGoogle Provider = "google"
	ProviderApple  Provider = "apple"
)

func (p Provider) Supported() bool { return p == ProviderGoogle || p == ProviderApple }
