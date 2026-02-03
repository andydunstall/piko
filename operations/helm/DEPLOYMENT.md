# Piko Helm Chart - Secure Deployment Guide

This guide walks you through deploying Piko on Kubernetes with TLS (via cert-manager) and JWT authentication.

## Prerequisites

1. Kubernetes cluster (v1.19+)
2. Helm 3.x
3. cert-manager installed with a ClusterIssuer named `letsencrypt-prod`
4. NGINX Ingress Controller (or adjust `ingress.className` for your ingress controller)

## Quick Start

### Step 1: Generate RSA Keys for JWT Authentication

```bash
# Create a directory for your keys (keep private key secure!)
mkdir -p ~/piko-keys && cd ~/piko-keys

# Generate RSA private key (2048 bits) - KEEP THIS SECURE
openssl genrsa -out jwt-private.pem 2048

# Extract the public key
openssl rsa -in jwt-private.pem -pubout -out jwt-public.pem

# View the public key (you'll need this for the Kubernetes secret)
cat jwt-public.pem
```

### Step 2: Create the JWT Public Key Secret in Kubernetes

```bash
# Create the namespace
kubectl create namespace piko

# Create the secret with the public key
kubectl create secret generic piko-jwt-rsa-public-key \
  --from-file=rsa-public-key=jwt-public.pem \
  --namespace piko
```

### Step 3: Create Your Values File

Create a file `my-values.yaml` with your specific configuration:

```yaml
# my-values.yaml - Customize these values for your deployment

replicaCount: 3

# TLS Configuration - uses your existing cert-manager
tls:
  enabled: true
  certManager:
    enabled: true
    issuerName: "letsencrypt-prod"  # Your existing ClusterIssuer
    issuerKind: "ClusterIssuer"

# Authentication - references the secret you created in Step 2
auth:
  enabled: true
  issuer: "your-company.com"  # Optional: JWT issuer to validate
  rsa:
    enabled: true
    existingSecret: "piko-jwt-rsa-public-key"
  proxy:
    enabled: true
    audience: "piko-proxy"
  upstream:
    enabled: true
    audience: "piko-upstream"
  admin:
    enabled: true
    audience: "piko-admin"

# Ingress - configure your public URLs
ingress:
  enabled: true
  className: "nginx"
  proxy:
    enabled: true
    host: "piko.yourdomain.com"           # Change to your domain
  upstream:
    enabled: true
    host: "upstream.piko.yourdomain.com"  # Change to your domain
  admin:
    enabled: false  # Enable if you need external admin access
```

### Step 4: Install the Chart

```bash
cd /path/to/piko/operations/helm

helm upgrade --install piko ./piko \
  --namespace piko \
  --create-namespace \
  -f my-values.yaml
```

### Step 5: Verify the Installation

```bash
# Check pods are running
kubectl get pods -n piko -w

# Check certificate was issued
kubectl get certificate -n piko

# Check the services
kubectl get svc -n piko

# Check ingress
kubectl get ingress -n piko
```

## Generating JWT Tokens

Your upstream services and clients need JWT tokens to authenticate. Here's how to generate them:

### Token Structure

Piko expects JWTs with this structure:

```json
{
  "iss": "your-company.com",
  "aud": "piko-upstream",       // or "piko-proxy" for clients
  "exp": 1735689600,
  "iat": 1735603200,
  "piko": {
    "endpoints": ["service-a", "service-b"]  // Optional: restrict endpoints
  }
}
```

### Generate Tokens (Node.js Example)

```javascript
const jwt = require('jsonwebtoken');
const fs = require('fs');

const privateKey = fs.readFileSync('jwt-private.pem');

// Token for an upstream service
const upstreamToken = jwt.sign(
  {
    piko: {
      endpoints: ['my-service']  // Only allow this endpoint
    }
  },
  privateKey,
  {
    algorithm: 'RS256',
    issuer: 'your-company.com',
    audience: 'piko-upstream',
    expiresIn: '30d'
  }
);

// Token for a client accessing services
const clientToken = jwt.sign(
  {
    piko: {
      endpoints: ['my-service']  // Only allow access to this endpoint
    }
  },
  privateKey,
  {
    algorithm: 'RS256',
    issuer: 'your-company.com',
    audience: 'piko-proxy',
    expiresIn: '24h'
  }
);

console.log('Upstream Token:', upstreamToken);
console.log('Client Token:', clientToken);
```

### Generate Tokens (Go Example)

```go
package main

import (
    "fmt"
    "os"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

func main() {
    keyData, _ := os.ReadFile("jwt-private.pem")
    privateKey, _ := jwt.ParseRSAPrivateKeyFromPEM(keyData)

    claims := jwt.MapClaims{
        "iss": "your-company.com",
        "aud": "piko-upstream",
        "exp": time.Now().Add(30 * 24 * time.Hour).Unix(),
        "iat": time.Now().Unix(),
        "piko": map[string]interface{}{
            "endpoints": []string{"my-service"},
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
    tokenString, _ := token.SignedString(privateKey)
    fmt.Println(tokenString)
}
```

## Connecting Services

### Upstream Service (Using Piko Agent)

```bash
piko agent http my-service 8080 \
  --server.url https://upstream.piko.yourdomain.com \
  --auth.token "$UPSTREAM_TOKEN"
```

### Client Requests

```bash
# Using x-piko-endpoint header
curl -H "Authorization: Bearer $CLIENT_TOKEN" \
     -H "x-piko-endpoint: my-service" \
     https://piko.yourdomain.com/api/data

# Using subdomain (requires wildcard DNS)
curl -H "Authorization: Bearer $CLIENT_TOKEN" \
     https://my-service.piko.yourdomain.com/api/data
```

## Advanced Configuration

### Disable Auth for Specific Endpoints

If you want to disable auth for proxy requests (public services):

```yaml
auth:
  enabled: true
  proxy:
    enabled: false  # No auth needed for clients
  upstream:
    enabled: true   # Upstreams still need auth
```

### Use JWKS Instead of Static Keys

For integration with identity providers (Auth0, Keycloak, etc.):

```yaml
auth:
  enabled: true
  rsa:
    enabled: false
  jwks:
    enabled: true
    endpoint: "https://your-idp.com/.well-known/jwks.json"
    cacheTTL: 1h
    timeout: 10s
```

### Enable Admin Ingress

```yaml
ingress:
  admin:
    enabled: true
    host: "admin.piko.yourdomain.com"
    annotations:
      nginx.ingress.kubernetes.io/whitelist-source-range: "10.0.0.0/8,192.168.0.0/16"
```

### Adjust Resources

```yaml
resources:
  limits:
    cpu: 2000m
    memory: 1Gi
  requests:
    cpu: 500m
    memory: 256Mi
```

## Troubleshooting

### Check Pod Logs

```bash
kubectl logs -f piko-0 -n piko
```

### Check Certificate Status

```bash
kubectl describe certificate piko-tls -n piko
```

### Verify TLS

```bash
openssl s_client -connect piko.yourdomain.com:443 -servername piko.yourdomain.com
```

### Check Cluster Status

```bash
kubectl exec -it piko-0 -n piko -- wget -qO- --no-check-certificate https://localhost:8002/status/cluster
```

### Check Upstreams

```bash
kubectl exec -it piko-0 -n piko -- wget -qO- --no-check-certificate https://localhost:8002/status/upstreams
```
