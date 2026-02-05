<p align="center">
  <img src="assets/images/logo.png?raw=true" width='40%'>
</p>

## Tim's Piko Fork ( [original repo](https://github.com/andydunstall/piko) )

---

# Piko Helm Chart Deployment Guide

This guide covers deploying Piko on Kubernetes with TLS, JWT authentication, and subdomain routing.

## Quick Deployment

### Step 1: Create HMAC Secret

Run the setup script:

```bash
cd operations/helm
./setup-hmac-secret.sh
```

This generates a secure HMAC secret and creates the Kubernetes secret. **Save the secret output** - you'll need it to generate tokens.

### Step 2: Configure Values

Edit `operations/helm/values.yaml` with your domain and settings. See `values-example.yaml` for reference.

Key settings:
- `ingress.proxy.host`: Your main domain (e.g., `piko.yourdomain.com`)
- `ingress.proxy.additionalHosts`: Add specific subdomains for valid certificates
- `auth.proxy.enabled`: Set to `false` for public access (no client auth required)

### Step 3: Deploy

```bash
cd operations/helm
helm upgrade --install piko ./piko \
  --namespace piko \
  --create-namespace \
  -f values.yaml
```

### Step 4: Verify

```bash
kubectl get pods -n piko
kubectl get certificate -n piko
kubectl wait --for=condition=Ready certificate/piko-tls -n piko --timeout=5m
```

## Generating JWT Tokens

### Generate Tokens for an Endpoint

Use the token generator script:

```bash
cd operations/helm

# Set your HMAC secret (from Step 1)
export HMAC_SECRET='your-secret-here'

# Generate tokens for an endpoint
./generate-token.sh my-service
```

This outputs:
- **UPSTREAM_TOKEN**: For services connecting to Piko
- **PROXY_TOKEN**: For clients accessing services (if proxy auth enabled)

### Generate Tokens for Multiple Users/Services

Run the script with different endpoint names:

```bash
# User 1: Only access to "api" endpoint
./generate-token.sh api

# User 2: Access to "web" endpoint  
./generate-token.sh web

# User 3: Access to "admin" endpoint
./generate-token.sh admin
```

To allow access to multiple endpoints, edit the script or use [jwt.io](https://jwt.io) with the payload:
```json
{
  "aud": "piko-upstream",
  "piko": { "endpoints": ["api", "web", "admin"] }
}
```

## Adding Multiple Subdomains

### Add a Subdomain

Use the helper script:

```bash
cd operations/helm
./add-subdomain.sh api.piko.yourdomain.com
```

This adds the subdomain to `values.yaml` and updates the certificate.

### Manual Method

1. Edit `operations/helm/values.yaml` and add subdomains to `ingress.proxy.additionalHosts`
2. Update the certificate:
   ```bash
   kubectl delete certificate piko-tls -n piko
   helm upgrade piko ./piko -n piko -f values.yaml
   ```
3. Wait for certificate: `kubectl wait --for=condition=Ready certificate/piko-tls -n piko`

**Note:** Wildcards (`*.piko.yourdomain.com`) are used for routing but not included in certificates (requires DNS-01 challenge). Add specific subdomains to get valid certificates.

## Connecting Services

### Install Piko CLI

Download from [releases](https://github.com/andydunstall/piko/releases) or build from source:
```bash
go build -o piko .
sudo mv piko /usr/local/bin/
```

### Connect a Service

```bash
export UPSTREAM_TOKEN="your-token-from-generate-token.sh"

piko agent http my-service 8080 \
  --connect.url https://upstream.piko.yourdomain.com \
  --connect.token "$UPSTREAM_TOKEN"
```

### Connect Multiple Services

Run the agent in separate terminals for each service:

```bash
# Terminal 1: API service
piko agent http api 3000 \
  --connect.url https://upstream.piko.yourdomain.com \
  --connect.token "$API_TOKEN"

# Terminal 2: Web service  
piko agent http web 8080 \
  --connect.url https://upstream.piko.yourdomain.com \
  --connect.token "$WEB_TOKEN"
```

## Accessing Services

### Using Subdomain Routing (Recommended)

```bash
curl https://my-service.piko.yourdomain.com/
curl https://api.piko.yourdomain.com/
```

### Using x-piko-endpoint Header

```bash
curl -H "x-piko-endpoint: my-service" \
     https://piko.yourdomain.com/
```

## Testing

Use the test script:

```bash
cd operations/helm
./test-piko.sh
```

## Configuration Summary

| Component | URL | Auth Required |
|-----------|-----|---------------|
| **Proxy** | `https://piko.yourdomain.com` | No (if `auth.proxy.enabled: false`) |
| **Upstream** | `https://upstream.piko.yourdomain.com` | Yes (JWT token) |
| **Admin** | Internal only (port 8002) | No (if `auth.admin.enabled: false`) |

## Troubleshooting

### Certificate Not Ready
```bash
kubectl describe certificate piko-tls -n piko
```

### Agent Connection Issues
- Verify token: `echo $UPSTREAM_TOKEN`
- Check URL: Must be `https://upstream.piko.yourdomain.com` (not the proxy URL)
- Check logs: `kubectl logs piko-0 -n piko`

### Service Not Accessible
- Verify agent is connected: `kubectl logs piko-0 -n piko | grep endpoint`
- Check DNS: `dig my-service.piko.yourdomain.com`

## Advanced Configuration

For detailed configuration options, see [operations/helm/DEPLOYMENT.md](operations/helm/DEPLOYMENT.md).

### Enable Proxy Authentication

Edit `values.yaml`:
```yaml
auth:
  proxy:
    enabled: true
    audience: "piko-proxy"
```

Then clients need tokens:
```bash
curl -H "Authorization: Bearer $PROXY_TOKEN" \
     https://my-service.piko.yourdomain.com/
```

### Use RSA Instead of HMAC

See [operations/helm/DEPLOYMENT.md](operations/helm/DEPLOYMENT.md) for RSA key setup.

---

## License

MIT License - see [LICENSE](LICENSE) for details.
