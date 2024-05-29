package piko

type connectOptions struct {
	apiKey string
	url    string
}

type ConnectOption interface {
	apply(*connectOptions)
}

type apiKeyOption string

func (o apiKeyOption) apply(opts *connectOptions) {
	opts.apiKey = string(o)
}

// WithAPIKey configures the API key to authenticate the client.
func WithAPIKey(key string) ConnectOption {
	return apiKeyOption(key)
}

type urlOption string

func (o urlOption) apply(opts *connectOptions) {
	opts.url = string(o)
}

// WithURL configures the Piko server URL. Such as
// 'https://piko.example.com:8001'.
func WithURL(url string) ConnectOption {
	return urlOption(url)
}
