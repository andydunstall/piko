#!/bin/bash
# Generate JWT tokens for Piko authentication
# Usage: ./generate-token.sh <endpoint-name> [hmac-secret]
#
# If HMAC_SECRET env var is set, uses that. Otherwise prompts for it.
# You can also pass it as second argument.

ENDPOINT="${1:-my-service}"
HMAC_SECRET="${2:-${HMAC_SECRET}}"

if [ -z "$HMAC_SECRET" ]; then
    echo "HMAC_SECRET not set. Options:"
    echo "  1. Set environment variable: export HMAC_SECRET='your-secret'"
    echo "  2. Pass as argument: ./generate-token.sh $ENDPOINT 'your-secret'"
    echo "  3. Get from Kubernetes: kubectl get secret piko-jwt-hmac-secret -n piko -o jsonpath='{.data.hmac-secret}' | base64 -d"
    exit 1
fi

if command -v node &> /dev/null; then
    node -e "
const crypto = require('crypto');

function base64url(str) {
    return Buffer.from(str).toString('base64')
        .replace(/=/g, '').replace(/\+/g, '-').replace(/\//g, '_');
}

function sign(payload, secret) {
    const header = { alg: 'HS256', typ: 'JWT' };
    const headerB64 = base64url(JSON.stringify(header));
    const payloadB64 = base64url(JSON.stringify(payload));
    const signature = crypto
        .createHmac('sha256', secret)
        .update(headerB64 + '.' + payloadB64)
        .digest('base64')
        .replace(/=/g, '').replace(/\+/g, '-').replace(/\//g, '_');
    return headerB64 + '.' + payloadB64 + '.' + signature;
}

const secret = '$HMAC_SECRET';
const now = Math.floor(Date.now() / 1000);

// Upstream token (30 days)
const upstreamToken = sign({
    aud: 'piko-upstream',
    exp: now + (30 * 24 * 60 * 60),
    iat: now,
    piko: { endpoints: ['$ENDPOINT'] }
}, secret);

// Proxy token (24 hours)
const proxyToken = sign({
    aud: 'piko-proxy', 
    exp: now + (24 * 60 * 60),
    iat: now,
    piko: { endpoints: ['$ENDPOINT'] }
}, secret);

console.log('=== Upstream Token (for services connecting to Piko) ===');
console.log('UPSTREAM_TOKEN=' + upstreamToken);
console.log('');
console.log('=== Proxy Token (for clients accessing services) ===');
console.log('PROXY_TOKEN=' + proxyToken);
"
else
    echo "Error: Node.js is required. Install Node.js or use jwt.io with:"
    echo "  Header: {\"alg\": \"HS256\", \"typ\": \"JWT\"}"
    echo "  Payload: {\"aud\": \"piko-upstream\", \"exp\": <timestamp>, \"piko\": {\"endpoints\": [\"$ENDPOINT\"]}}"
    echo "  Secret: $HMAC_SECRET"
    exit 1
fi
