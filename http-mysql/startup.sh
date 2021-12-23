#!/bin/bash

# For local development and test only. See documentation.
if [[ -v GOOGLE_APPLICATION_CREDENTIALS ]]; then
  gcloud auth activate-service-account --key-file=$GOOGLE_APPLICATION_CREDENTIALS
fi

_saveGCS()
{
  echo "save file to gcs ${BUCKET}"
  tar -cf ${ARCHIVE_NAME} /var/lib/mysql/*
  gzip ${ARCHIVE_NAME}
  gcloud alpha storage cp -r ${ARCHIVE_NAME}.gz gs://${BUCKET}/
  exit 0
}

# Catch the instance shut down
trap _saveGCS SIGINT SIGTERM


# Deactivate the CRC hash check, missing crc32c lib
gcloud config set storage/check_hashes never

mkdir -p /var/lib/mysql

gcloud alpha storage cp -r gs://${BUCKET}/${ARCHIVE_NAME}.gz .
if [ -f ${ARCHIVE_NAME}.gz ]; then
  gunzip ${ARCHIVE_NAME}.gz
  tar -xf  ${ARCHIVE_NAME} -C /
  rm ${ARCHIVE_NAME}
fi

# Run MySQL service
source docker-entrypoint.sh
_main "mysqld" &

# Run the Webserver
/main
