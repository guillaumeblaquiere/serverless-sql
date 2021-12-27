#!/bin/bash

if [[ -v URL ]]; then
  URL="--url=${URL}"
fi

if [[ -v NO_TLS ]]; then
  NO_TLS="-no-tls=${NO_TLS}"
fi

if [[ -v PORT ]]; then
  PORT="--url=${PORT}"
fi

echo "use the parameters ${URL} ${NO_TLS} ${PORT}"

# Run the proxy
/serverless-db-proxy ${URL} ${NO_TLS} ${PORT} &

# Run the CMD
$@
