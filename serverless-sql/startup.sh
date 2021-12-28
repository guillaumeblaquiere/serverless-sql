#!/bin/bash

# For local development and test only. See documentation.
if [[ -v GOOGLE_APPLICATION_CREDENTIALS ]]; then
  gcloud auth activate-service-account --key-file=${GOOGLE_APPLICATION_CREDENTIALS}
fi

#If the root password start by sm:// that means the format is the following
# projects/<PROJECT_ID>/secrets/<SECRET_NAME>/versions/<VERSION>
# And require a get from Secret Manager
if [[ -v ROOT_PASSWORD ]]; then
  if [[ ${ROOT_PASSWORD} == sm://* ]]; then
    PROJECT_ID=$(echo ${ROOT_PASSWORD} | cut -d "/" -f 4)
    SECRET_NAME=$(echo ${ROOT_PASSWORD} | cut -d "/" -f 6)
    VERSION=$(echo ${ROOT_PASSWORD} | cut -d "/" -f 8)

    MYSQL_ROOT_PASSWORD=$(gcloud secrets versions access ${VERSION} --secret=${SECRET_NAME} --project=${PROJECT_ID})
    if [[ $? != 0 ]]; then
      echo "Invalid secret (bad formatted or not exist): ${ROOT_PASSWORD}"
      exit 2
    fi
  else
    MYSQL_ROOT_PASSWORD=${ROOT_PASSWORD}
  fi
else
  echo "ROOT_PASSWORD environment variable must be set to define the database root password."
  exit 1
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