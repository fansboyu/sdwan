# SD-WAN Controller

`sdwan` 鏄竴涓渶灏忕増 Tailscale-like 浜у搧楠ㄦ灦銆傜涓€闃舵鑱氱劍杞欢鎺у埗闈㈠拰瀹㈡埛绔帴鍏ュ崗璁細绠＄悊鍛橀€氳繃閭娉ㄥ唽鐧诲綍锛屽垱寤哄鎴峰悗鑾峰緱 Join Token锛汱inux/Windows 瀹㈡埛绔娇鐢?Join Token 娉ㄥ唽锛孋ontroller 涓哄鎴风鍒嗛厤 Overlay 铏氭嫙 IP锛屽苟閫氳繃 HTTP polling 涓嬪彂 Netmap銆?
褰撳墠鐗堟湰锛歚v1.1.0`

榛樿鎺у埗鍣ㄥ煙鍚嶏細

```text
controller.englishlisten.cn
```

## 浜у搧鐩爣

绗竴鐗堢洰鏍囨槸璺戦€氭渶灏忛棴鐜細

- 绠＄悊鍛樺彧閫氳繃閭娉ㄥ唽鍜岀櫥褰曘€?- 绠＄悊鍛樺垱寤哄鎴枫€?- 姣忎釜瀹㈡埛榛樿鍒嗛厤涓€涓?`/28` Overlay 鍦板潃姹狅紝榛樿鏀寔 16 涓鎴风銆?- 绯荤粺涓哄鎴风敓鎴?Join Token銆?- Linux/Windows 瀹㈡埛绔娇鐢?Join Token 娉ㄥ唽銆?- Controller 涓哄鎴风鍒嗛厤铏氭嫙 IP銆?- 瀹㈡埛绔€氳繃 HTTP polling 涓婃姤蹇冭烦銆丒ndpoint 鍜岀増鏈俊鎭€?- 瀹㈡埛绔媺鍙?Netmap锛岀敤浜庣敓鎴?WireGuard peer 閰嶇疆銆?- 鍚屼竴瀹㈡埛涓嬮粯璁ゅ叏浜掗€氥€?
绗竴鐗堟殏涓嶅仛锛?
- WebSocket 鎺ㄩ€併€?- 澶嶆潅 ACL銆?- MagicDNS銆?- Exit Node銆?- Subnet Router銆?- Relay/DERP 杞彂銆?- 澶?Controller 楂樺彲鐢ㄣ€?- 鑷姩瀹㈡埛绔崌绾с€?- 鐢ㄦ埛鍚嶃€佹墜鏈哄彿銆佺涓夋柟 OAuth 鐧诲綍銆?- 閭楠岃瘉鐮佸拰 SMTP 鍙戜俊銆?
## 鎶€鏈矾绾?
鍚庣鎺у埗鍣細

- Go
- 鏍囧噯搴?`net/http`
- PostgreSQL
- sqlc 椋庢牸鏌ヨ灞?- `pgx/v5`
- Token 鏄庢枃鍙繑鍥炰竴娆★紝鏁版嵁搴撳彧淇濆瓨 SHA-256 hash
- Docker Compose 閮ㄧ讲

鍓嶇涓绘帶椤甸潰锛?
- Vue 3
- Vite
- JavaScript
- 鍘熺敓 CSS
- 榛樿涓枃灞曠ず锛屽彲鍒囨崲鑻辨枃
- 鏋勫缓鍚庣敱 nginx 瀹瑰櫒鎵樼

STUN锛?
- coturn
- STUN-only 妯″紡
- UDP 3478
- 榛樿鍦板潃锛歚stun:controller.englishlisten.cn:3478`

閮ㄧ讲鏂瑰紡锛?
- 鏈粨搴撳彧鎻愪緵 Docker Compose銆?- 鏈粨搴撲笉鍐呯疆 Caddy銆?- 鐢熶骇鐜鐢辨湇鍔″櫒涓婄嫭绔嬮儴缃茬殑 Caddy/Nginx 鍋氭湇鍔″垎閰嶃€?
## 鏁翠綋鏋舵瀯

```text
Admin Web(Vue/nginx) ---> Controller(Go) ---> PostgreSQL

Linux/Windows Client
      |
      | HTTPS polling / netmap
      v
Controller

Client <---- WireGuard P2P ----> Client
```

鏈湴 Docker Compose 鏆撮湶锛?
```text
涓绘帶椤甸潰锛歨ttp://localhost:8081
Controller API锛歨ttp://localhost:18080
STUN锛歶dp://localhost:3478
```

鐢熶骇鏃跺彲浠ョ敱浣犳湇鍔″櫒涓婄殑鐙珛 Caddy/Nginx 鑷鍒嗘祦锛?
```text
https://controller.englishlisten.cn/        -> web:80
https://controller.englishlisten.cn/api/*   -> controller:8080
https://controller.englishlisten.cn/admin/* -> controller:8080
udp://controller.englishlisten.cn:3478      -> stun:3478/udp
```

## 鍦板潃鍒嗛厤

鍏ㄥ眬 Overlay 鍦板潃姹狅細

```text
100.64.0.0/10
```

姣忎釜瀹㈡埛榛樿鍒嗛厤涓€涓?`/28`锛?
```text
瀹㈡埛 A锛?00.64.0.0/28
瀹㈡埛 B锛?00.64.0.16/28
瀹㈡埛 C锛?00.64.0.32/28
```

姣忎釜瀹㈡埛榛樿瀹归噺锛?
```text
16 涓鎴风
```

璁惧鎸?`/32` 鍗曞湴鍧€鍒嗛厤锛?
```text
client-1 -> 100.64.0.0/32
client-2 -> 100.64.0.1/32
...
```

### 濡備綍淇濊瘉瀹㈡埛鍦板潃姹犱笉閲嶅

褰撳墠浣跨敤涓夊眰淇濇姢锛?
```text
1. 鏁版嵁搴撳敮涓€绾︽潫
   customers.address_cidr UNIQUE

2. 浜嬪姟绾у垎閰嶉攣
   鍒涘缓瀹㈡埛鏃朵娇鐢?PostgreSQL pg_advisory_xact_lock銆?   鍚屼竴鏃堕棿鍙湁涓€涓簨鍔¤兘璁＄畻鍜屽啓鍏ユ柊鐨勫鎴峰湴鍧€姹犮€?
3. 瀹㈡埛鍐呰澶囧敮涓€绾︽潫
   devices(customer_id, virtual_ip) UNIQUE
   闃叉鍚屼竴瀹㈡埛鍐呴噸澶嶅垎閰嶈澶?IP銆?```

褰撳墠瀹㈡埛鍦板潃姹犲垎閰嶆祦绋嬶細

```text
1. 寮€鍚暟鎹簱浜嬪姟銆?2. 鑾峰彇 PostgreSQL advisory transaction lock銆?3. 鏌ヨ鏈€鍚庝竴涓鎴?CIDR銆?4. 鍚戝悗鍋忕Щ 16 涓湴鍧€銆?5. 鐢熸垚涓嬩竴涓?/28銆?6. 鍐欏叆 customers銆?7. 鍐欏叆 join_tokens銆?8. 鎻愪氦浜嬪姟銆?```

杩欐牱鍗充娇涓や釜绠＄悊鍛樺悓鏃跺垱寤哄鎴凤紝涔熶笉浼氳绠楀嚭鍚屼竴涓?`/28` 骞舵垚鍔熷啓鍏ャ€?
## Token 妯″瀷

绯荤粺浣跨敤涓夌被 Token锛?
```text
Admin Token:
  绠＄悊鍛樼櫥褰曞悗鑾峰緱銆?  鐢ㄤ簬璁块棶 /admin/* 绠＄悊鎺ュ彛銆?  褰撳墠鏈夋晥鏈熶负 30 澶┿€?
Join Token:
  瀹㈡埛绾ф帴鍏?Token銆?  鐢ㄤ簬瀹㈡埛绔娆℃敞鍐屻€?
Device Token:
  璁惧绾?Token銆?  娉ㄥ唽鎴愬姛鍚庤繑鍥炵粰瀹㈡埛绔€?  鍚庣画 poll 鍜?netmap 璇锋眰浣跨敤瀹冮壌鏉冦€?```

瀹夊叏绾︽潫锛?
- Token 鏄庢枃鍙繑鍥炰竴娆°€?- 鏁版嵁搴撳彧淇濆瓨 Token hash銆?- Controller 涓嶄繚瀛樺鎴风 WireGuard 绉侀挜銆?- Controller 鍙繚瀛樺鎴风 WireGuard 鍏挜銆?
## 鐗堟湰绛栫暐

Controller 鐗堟湰锛?
```text
v1.1.0
```

瀹㈡埛绔吋瀹归厤缃細

```text
MIN_SUPPORTED_CLIENT_VERSION=v1.1.0
LATEST_CLIENT_VERSION=v1.1.0
```

绗竴鐗堝彧鍋氱増鏈噰闆嗗拰鍗囩骇鎻愮ず锛屼笉鍋氳嚜鍔ㄥ崌绾с€?
## 鐩綍缁撴瀯

```text
cmd/controller/        Controller 鍚姩鍏ュ彛
cmd/agent/             Linux Agent CLI 楠岃瘉鐗?internal/app/          璁よ瘉銆佸鎴枫€乀oken銆佽澶囨敞鍐屻€乸olling銆乶etmap
internal/agent/        Agent API 瀹㈡埛绔€佹湰鍦伴厤缃€乄ireGuard 閰嶇疆娓叉煋
internal/config/       鐜鍙橀噺閰嶇疆
internal/httpapi/      HTTP 璺敱鍜?JSON API
internal/storage/      PostgreSQL 杩炴帴鍜岃縼绉诲叆鍙?internal/storage/sqlc/ sqlc 椋庢牸鏌ヨ浠ｇ爜
db/migrations/         PostgreSQL 琛ㄧ粨鏋勮縼绉?db/queries/            sqlc 鏌ヨ瀹氫箟
deploy/coturn/         coturn STUN-only 閰嶇疆
web/                   Vue 涓绘帶椤甸潰
```

## 鏈湴杩愯

鍚姩锛?
```bash
docker compose up --build
```

鏌ョ湅鏈嶅姟锛?
```bash
docker compose ps
```

鏌ョ湅鏃ュ織锛?
```bash
docker compose logs -f controller
docker compose logs -f stun
```

鍋滄锛?
```bash
docker compose down
```

娓呯悊鏁版嵁搴撳嵎锛?
```bash
docker compose down -v
```

鏈湴璁块棶锛?
```text
涓绘帶椤甸潰锛歨ttp://localhost:8081
Controller API锛歨ttp://localhost:18080
```

## 鑷缓 STUN 鏈嶅姟

椤圭洰宸茬粡鍦?Docker Compose 涓泦鎴?coturn锛屼綔涓?STUN-only 鏈嶅姟杩愯銆?
閰嶇疆鏂囦欢锛?
```text
deploy/coturn/turnserver.conf
```

鐢熶骇鐜闇€瑕佺‘璁わ細

```text
controller.englishlisten.cn 瑙ｆ瀽鍒伴儴缃叉湇鍔″櫒鍏綉 IP
鏈嶅姟鍣ㄥ畨鍏ㄧ粍寮€鏀?UDP 3478
鎿嶄綔绯荤粺闃茬伀澧欏紑鏀?UDP 3478
Caddy/Nginx 涓嶄唬鐞?STUN锛孲TUN 鐩存帴鏆撮湶 UDP 3478
```

楠岃瘉 STUN锛?
```bash
docker run --rm --network host coturn/coturn:4.6-alpine \
  turnutils_stunclient controller.englishlisten.cn 3478
```

鐪嬪埌绫讳技 `Mapped address` 鐨勮緭鍑猴紝璇存槑 STUN 鍙互杩斿洖鍏綉鏄犲皠鍦板潃銆?
## 鐜鍙橀噺

```text
DATABASE_URL
  PostgreSQL 杩炴帴涓层€?
LISTEN_ADDR
  Controller 鐩戝惉鍦板潃锛岄粯璁?:8080銆?
CONTROLLER_URL
  瀵瑰鎴风灞曠ず鐨勬帶鍒跺櫒 URL锛岄粯璁?https://controller.englishlisten.cn銆?
DEFAULT_CUSTOMER_MAX_DEVICES
  姣忎釜瀹㈡埛榛樿璁惧鏁帮紝榛樿 16銆?
POLL_INTERVAL_SECONDS
  瀹㈡埛绔?polling 闂撮殧锛岄粯璁?15銆?
MIN_SUPPORTED_CLIENT_VERSION
  鏈€浣庢敮鎸佸鎴风鐗堟湰锛岄粯璁?v1.1.0銆?
LATEST_CLIENT_VERSION
  鏈€鏂板鎴风鐗堟湰锛岄粯璁?v1.1.0銆?
STUN_SERVERS
  Controller 涓嬪彂缁欏鎴风鐨?STUN 鏈嶅姟鍒楄〃锛屽涓湴鍧€鐢ㄩ€楀彿鍒嗛殧銆?  榛樿 stun:controller.englishlisten.cn:3478銆?
LOG_LEVEL
  鏃ュ織绛夌骇锛歞ebug/info/warn/error锛岄粯璁?info銆?```

## API 鎺ュ彛

### GET /api/v1/server/version

```bash
curl http://localhost:18080/api/v1/server/version
```

### POST /admin/auth/register

```bash
curl -X POST http://localhost:18080/admin/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"password123"}'
```

### POST /admin/auth/login

```bash
curl -X POST http://localhost:18080/admin/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"password123"}'
```

### GET /admin/auth/me

```bash
curl http://localhost:18080/admin/auth/me \
  -H "Authorization: Bearer sdwan_admin_xxx"
```

### POST /admin/customers

```bash
curl -X POST http://localhost:18080/admin/customers \
  -H "Authorization: Bearer sdwan_admin_xxx" \
  -H "Content-Type: application/json" \
  -d '{"name":"demo"}'
```

### GET /admin/customers

```bash
curl http://localhost:18080/admin/customers \
  -H "Authorization: Bearer sdwan_admin_xxx"
```

### POST /api/v1/devices/register

```bash
curl -X POST http://localhost:18080/api/v1/devices/register \
  -H "Content-Type: application/json" \
  -d '{
    "join_token": "sdwan_join_xxx",
    "hostname": "linux-01",
    "os": "linux",
    "arch": "amd64",
    "public_key": "wireguard-public-key",
    "client_version": "v1.1.0"
  }'
```

### POST /api/v1/devices/poll

```bash
curl -X POST http://localhost:18080/api/v1/devices/poll \
  -H "Authorization: Bearer sdwan_device_xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "current_netmap_version": 1,
    "client_version": "v1.1.0",
    "endpoints": [
      {"type":"lan","addr":"192.168.1.10:41641","source":"local"}
    ]
  }'
```

### GET /api/v1/netmap

```bash
curl http://localhost:18080/api/v1/netmap \
  -H "Authorization: Bearer sdwan_device_xxx"
```

## 涓绘帶椤甸潰

褰撳墠 Vue 涓绘帶椤甸潰鍖呭惈锛?
- 榛樿涓枃灞曠ず銆?- 鍙垏鎹㈣嫳鏂囧睍绀恒€?- 閭娉ㄥ唽銆?- 閭鐧诲綍銆?- Controller 鐗堟湰灞曠ず銆?- 鍒涘缓瀹㈡埛銆?- 灞曠ず瀹㈡埛 Join Token銆?- 瀹㈡埛鍒楄〃銆?
## Linux 瀹㈡埛绔獙璇?
鏋勫缓 Linux Agent锛?
```bash
GOOS=linux GOARCH=amd64 go build -o sdwan-agent ./cmd/agent
```

Linux 渚濊禆锛?
```bash
sudo apt update
sudo apt install -y wireguard-tools
```

娉ㄥ唽璁惧锛?
```bash
sudo ./sdwan-agent register \
  --controller http://localhost:18080 \
  --join-token sdwan_join_xxx
```

娓叉煋 WireGuard 閰嶇疆锛?
```bash
sudo ./sdwan-agent render --out /tmp/sdwan0.conf
cat /tmp/sdwan0.conf
```

## 楠岃瘉鍛戒护

```bash
go test ./...

cd web
npm run build

docker compose config
docker compose build
```

## 鍚庣画寮€鍙戣矾绾?
1. 涓绘帶椤甸潰鎷嗗垎涓?`api/`銆乣views/`銆乣components/`銆?2. 澧炲姞瀹㈡埛璇︽儏椤点€?3. 澧炲姞璁惧鍒楄〃椤点€?4. 澧炲姞璁惧璇︽儏椤碉紝灞曠ず铏氭嫙 IP銆佸鎴风鐗堟湰銆佹渶鍚庡湪绾挎椂闂淬€丒ndpoint銆?5. 澧炲姞璁惧绂佺敤/鍒犻櫎鎺ュ彛銆?6. Linux Agent 澧炲姞 daemon/systemd 妯″紡銆?7. Linux Agent 澧炲姞鑷姩 STUN Endpoint 鎺㈡祴銆?8. 澧炲姞 Windows Agent銆?9. 澧炲姞 Relay銆?10. 鍐嶈€冭檻 WebSocket 鎺ㄩ€佹浛鎹?polling銆?
