#!/bin/sh
set -eu

PORT="${PORT:-8080}"
ENVOY_ADMIN_HOST="${ENVOY_ADMIN_HOST:-127.0.0.1}"
ENVOY_ADMIN_PORT="${ENVOY_ADMIN_PORT:-9901}"
UPSTREAM_HOST="${UPSTREAM_HOST:-127.0.0.1}"
UPSTREAM_PORT="${UPSTREAM_PORT:-50051}"
CORS_ALLOW_ORIGIN_REGEX="${CORS_ALLOW_ORIGIN_REGEX:-.*}"
ENVOY_LOG_LEVEL="${ENVOY_LOG_LEVEL:-info}"
ENVOY_MAX_DOWNSTREAM_CONNECTIONS="${ENVOY_MAX_DOWNSTREAM_CONNECTIONS:-1024}"

if [ "${CORS_ALLOW_ORIGINS:-}" != "" ]; then
	normalized_list="$(printf '%s' "$CORS_ALLOW_ORIGINS" | tr -d '\r' | tr ',' '\n' | sed -e 's/[[:space:]]//g' -e 's#^\(https\?://[^/]*\)/.*$#\1#' -e 's#/$##' -e '/^$/d')"
	escaped_list="$(printf '%s\n' "$normalized_list" | sed -e 's/[.[\\^$*+?(){|}]/\\&/g')"
	joined="$(printf '%s\n' "$escaped_list" | tr '\n' '|' | sed -e 's/|$//')"
	if [ "$joined" != "" ]; then
		CORS_ALLOW_ORIGIN_REGEX="^($joined)$"
	fi
fi

export PORT
export ENVOY_ADMIN_HOST
export ENVOY_ADMIN_PORT
export UPSTREAM_HOST
export UPSTREAM_PORT
export CORS_ALLOW_ORIGIN_REGEX
export ENVOY_MAX_DOWNSTREAM_CONNECTIONS

envsubst < /etc/envoy/envoy.yaml.tmpl > /tmp/envoy.yaml

exec envoy -c /tmp/envoy.yaml --log-level "${ENVOY_LOG_LEVEL}"
