# BridgeAI Quick Start

One API key. Multiple AI models. Automatic fallback.

**Base URL:** `https://bridgeai-production-e475.up.railway.app/v1`

---

## 1. Make your first call

```bash
curl https://bridgeai-production-e475.up.railway.app/v1/chat/completions \
  -H "Authorization: Bearer <your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "smart-fast",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

## 2. Drop-in replacement for OpenAI SDK

**Python**
```python
from openai import OpenAI

client = OpenAI(
    base_url="https://bridgeai-production-e475.up.railway.app/v1",
    api_key="<your-api-key>"
)

resp = client.chat.completions.create(
    model="smart-fast",
    messages=[{"role": "user", "content": "Hello"}]
)
print(resp.choices[0].message.content)
```

**Node.js**
```javascript
import OpenAI from "openai";

const client = new OpenAI({
  baseURL: "https://bridgeai-production-e475.up.railway.app/v1",
  apiKey: "<your-api-key>",
});

const resp = await client.chat.completions.create({
  model: "smart-fast",
  messages: [{ role: "user", content: "Hello" }],
});
console.log(resp.choices[0].message.content);
```

## 3. Models

| Alias | Description |
|-------|-------------|
| `smart-fast` | Fast and cheap — best for high-volume tasks |
| `smart-quality` | Higher quality — best for complex tasks |

BridgeAI routes to the best available provider automatically. If one fails, it falls back to the next — no changes needed on your end.

## 4. Streaming

Add `"stream": true` to any request:

```bash
curl -N https://bridgeai-production-e475.up.railway.app/v1/chat/completions \
  -H "Authorization: Bearer <your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "smart-fast",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": true
  }'
```

## 5. Bring your own keys (BYOK)

Use your own provider API keys — BridgeAI handles routing, you keep control of billing:

```bash
curl https://bridgeai-production-e475.up.railway.app/v1/chat/completions \
  -H "Authorization: Bearer <your-api-key>" \
  -H "X-Groq-Key: gsk_your_groq_key" \
  -H "Content-Type: application/json" \
  -d '{"model": "smart-fast", "messages": [{"role": "user", "content": "Hello"}]}'
```

## 6. Request tracing

Every response includes a `X-BridgeAI-Request-Id` header for debugging:

```
X-BridgeAI-Request-Id: req_xxxxxxxx
```

---

Questions? Email: **support@bridgeai.com**
