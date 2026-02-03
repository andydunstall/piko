#!/bin/bash
# Generate JWT tokens for Piko authentication
# Requires: npm install -g jwt-cli  OR use the node script below

HMAC_SECRET="HCO4vKXDgtqiTGPWb3W8Yxd6XPXs+T8RFPMhZBAuUxY="
ENDPOINT="${1:-my-service}"

# Using Node.js (if available)
if command -v node &> /dev/null; then
    echo "=== Upstream Token (for services connecting to Piko) ==="
    node -e "
const crypto = require('crypto');

function base64url(str) {
    return Buffer.from(str).toString('base64')
        .replace(/=/g, '')
        .replace(/\+/g, '-')
        .replace(/\//g, '_');
}

function sign(payload, secret) {
    const header = { alg: 'HS256', typ: 'JWT' };
    const headerB64 = base64url(JSON.stringify(header));
    const payloadB64 = base64url(JSON.stringify(payload));
    const signature = crypto
        .createHmac('sha256', secret)
        .update(headerB64 + '.' + payloadB64)
        .digest('base64')
        .replace(/=/g, '')
        .replace(/\+/g, '-')
        .replace(/\//g, '_');
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

console.log('UPSTREAM_TOKEN=' + upstreamToken);
console.log('');
console.log('=== Proxy Token (for clients accessing services) ===');
console.log('PROXY_TOKEN=' + proxyToken);
"
else
    echo "Node.js not found. Install it or use the Python alternative below."
    echo ""
    echo "Or use this online tool: https://jwt.io"
    echo "Header: {\"alg\": \"HS256\", \"typ\": \"JWT\"}"
    echo "Payload for upstream: {\"aud\": \"piko-upstream\", \"exp\": $(($(date +%s) + 2592000)), \"piko\": {\"endpoints\": [\"$ENDPOINT\"]}}"
    echo "Payload for proxy: {\"aud\": \"piko-proxy\", \"exp\": $(($(date +%s) + 86400)), \"piko\": {\"endpoints\": [\"$ENDPOINT\"]}}"
    echo "Secret: $HMAC_SECRET"
fi
