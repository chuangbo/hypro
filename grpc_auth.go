package hypro

import "context"

// grpcAuth implements credentials.PerRPCCredentials
type grpcAuth struct {
	host, token *string
	insecure    *bool
}

func (a *grpcAuth) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"host":  *a.host,
		"token": *a.token,
	}, nil
}

func (a *grpcAuth) RequireTransportSecurity() bool {
	return !*a.insecure
}
