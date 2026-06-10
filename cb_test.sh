#!/bin/bash
# Crate & Barrel Gemini Agent - test helper
# Usage: ./cb_test.sh "your prompt"

get_token() {
  curl -sk -X POST 'https://qa-www.crateandbarrel.com/header/get-gsa-token' \
    -H "IsCorporateRequest: true" -H "Accept: application/json" -H "Content-Type: application/json" \
    -H "Origin: https://qa-www.crateandbarrel.com" -H "Referer: https://qa-www.crateandbarrel.com/" \
    -H "User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36" \
    -d '{}'
}

ask() {
  local prompt="$1"
  local session_id="${2:-}"
  RESP=$(get_token)
  TOKEN=$(echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['Token'])")
  SID=$(echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['SessionId'])")
  [ -n "$session_id" ] && SID="$session_id"

  echo "=== SESSION: $SID ==="
  echo "=== PROMPT: $prompt ==="
  echo ""

  curl -sk -X POST 'https://agenticapplications.googleapis.com/v1/sales:executeChat?alt=sse' \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -H "Accept: text/event-stream" \
    --max-time 30 \
    -d "{
      \"clientId\": \"OfhPJM8d2IZHTB88\",
      \"sessionId\": \"$SID\",
      \"inputs\": [{\"text\": $(echo "$prompt" | python3 -c "import sys,json; print(json.dumps(sys.stdin.read().strip()))")}]
    }" | python3 -c "
import sys,json
for line in sys.stdin:
    line=line.strip()
    if line.startswith('data: '):
        try:
            d=json.loads(line[6:])
            r=d.get('response',{})
            if 'textBlock' in r:
                for el in r['textBlock'].get('item',{}).get('elements',[]):
                    print(el.get('text',''), end='', flush=True)
            elif 'thought' in r:
                for p in r['thought'].get('parts',[]):
                    print('[THOUGHT] '+p.get('text',''))
            elif 'toolUse' in r:
                print('[TOOL_USE] '+json.dumps(r['toolUse']))
            elif 'toolResult' in r:
                print('[TOOL_RESULT] '+json.dumps(r['toolResult'])[:500])
        except: pass
print()
"
}

ask "$1" "$2"
