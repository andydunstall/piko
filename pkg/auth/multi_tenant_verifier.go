package auth

type MultiTenantVerifier struct {
	defaultVerifier Verifier
	tenantVerifiers map[string]Verifier
}

func NewMultiTenantVerifier(
	defaultVerifier Verifier,
	tenantVerifiers map[string]Verifier,
) *MultiTenantVerifier {
	return &MultiTenantVerifier{
		defaultVerifier: defaultVerifier,
		tenantVerifiers: tenantVerifiers,
	}
}

func (v *MultiTenantVerifier) Verify(token string, tenantID string) (*Token, error) {
	if tenantID == "" {
		if len(v.tenantVerifiers) != 0 {
			// If tenants are configured, the default tenant is disabled.
			return nil, ErrUnknownTenant
		}
		return v.defaultVerifier.Verify(token)
	}

	if v.tenantVerifiers == nil {
		return nil, ErrUnknownTenant
	}

	verifier, ok := v.tenantVerifiers[tenantID]
	if !ok {
		return nil, ErrUnknownTenant
	}

	t, err := verifier.Verify(token)
	if err != nil {
		return nil, err
	}
	t.TenantID = tenantID
	return t, nil
}
