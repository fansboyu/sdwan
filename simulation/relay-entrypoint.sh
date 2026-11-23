#!/bin/sh
set -eu
umask 077
cd /etc/sdwan
controller=http://controller:8080
iface=sdwan-relay
if [ ! -s relay.key ]; then wg genkey > relay.key; fi
ip link add "$iface" type wireguard
wg set "$iface" listen-port 51873 private-key relay.key
ip addr add 100.254.253.1/32 dev "$iface"
ip link set "$iface" up
ip route replace 100.64.240.0/24 dev "$iface"
ip route replace 192.168.250.0/24 dev "$iface"
iptables -A FORWARD -i "$iface" -o "$iface" -j ACCEPT

if [ ! -s relay-agent.json ]; then
  auth=$(curl -fsS --retry 10 --retry-connrefused -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg password "$LAB_ADMIN_PASSWORD" '{email:"lab-admin@sdwan.test",password:$password}')" "$controller/admin/auth/login" | jq -er .token)
  request=$(jq -nc --arg key "$(wg pubkey < relay.key)" '{name:"Docker lab relay",public_key:$key,endpoint:"10.254.10.250:51873",virtual_ip:"100.254.253.1"}')
  result=$(curl -fsS -H "Authorization: Bearer $auth" -H 'Content-Type: application/json' -d "$request" "$controller/admin/relays")
  relay_id=$(printf '%s' "$result" | jq -er .relay.id)
  token=$(printf '%s' "$result" | jq -er .relay_token)
  curl -fsS -X POST -H "Authorization: Bearer $auth" "$controller/admin/relays/$relay_id/enable" >/dev/null
  jq -nc --arg url "$controller" --arg token "$token" '{controller_url:$url,relay_token:$token,interface_name:"sdwan-relay",sync_interval_seconds:5}' > relay-agent.json.tmp
  mv relay-agent.json.tmp relay-agent.json
fi
exec sdwan-relay-agent
