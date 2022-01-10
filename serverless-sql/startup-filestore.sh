#!/usr/bin/env bash

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

mkdir -p /var/lib/mysql

echo "Mounting Cloud Filestore."
mount --verbose -o nolock ${FILESTORE_IP_ADDRESS}:/${FILE_SHARE_NAME}/mysql /var/lib/mysql
echo "Mounting completed."

# Run MySQL service
source docker-entrypoint.sh
_main "mysqld" &

# Run the Webserver
/main