package piko

type connectOptions struct {
	token string
	url   string
}

type ConnectOption interface {
	apply(*connectOptions)
}

type tokenOption string

func (o tokenOption) apply(opts *connectOptions) {
	opts.token = string(o)
}

// WithToken configures the API key to authenticate the client.
func WithToken(key string) ConnectOption {
	return tokenOption(key)
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
