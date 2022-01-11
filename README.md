# Overview

That proof of concept use Cloud Run to deploy a SQL database on demand. That solution prevent to use Cloud SQL for 
development database or for non-critical and low usage production database.

[//]: # (TODO -> link to Medium Article)

# Technical overview

There is 2 parts at that solution

* the serverless sql container
* the serverless database proxy

# Quick links

* Packaged container to deploy on Cloud Run (or elsewhere): 
  * Cloud Storage option: `us-central1-docker.pkg.dev/gblaquiere-dev/public/serverless-sql`
  * Cloud Filestore option: `us-central1-docker.pkg.dev/gblaquiere-dev/public/serverless-sql-filestore`
* Compiled serverless db proxy
  * [Windows/amd64](https://storage.googleapis.com/serverless-db-proxy/win64/serverless-db-proxy.exe)
  * [Linux/amd64](https://storage.googleapis.com/serverless-db-proxy/linux64/serverless-db-proxy)
  * [Darwin/amd64](https://storage.googleapis.com/serverless-db-proxy/darwin64/serverless-db-proxy)
* Serverless DB proxy container
  * `us-central1-docker.pkg.dev/gblaquiere-dev/public/serverless-db-proxy`
* Serverless DB proxy startup script
  * [Linux/amd64](https://storage.googleapis.com/serverless-db-proxy/startup.sh)

## Serverless sql container

That container is composed of 2 parts

* The database engine itself that run as background process in the container
* The HTTP interface that serve the traffic.

The process is the following
1. The database engine starts when the container starts. The data are loaded from Cloud Storage, gunzip and untar
2. The database engine runs in the container and use the data loaded in the memory of the container
3. When the container is stopped (signals TERM or INT received), the data are tar and gzip and sent to Cloud Storage

The use of the HTTP endpoint is described in the "Communication between the 2 parties" section

## Serverless database proxy

The proxy listen the TCP communication and act as proxy to forward them to the serverless sql Cloud Run service.

## Communication between the 2 parts

Cloud Run only accept HTTP connection and disallow TCP connection. That's why, the TCP communication with the database 
must be wrapped in HTTP protocol.

For that, the proxy and the Serverless SQL container open HTTP/2 connection and use bidirectional streaming to 
exchange data. The proxy initiate the connection.

# Deployment of the database

There is 2 flavors:
1. Use Cloud Storage to load and store the database data
2. Use Cloud Filestore to mount a NFS share and use it as data storage

## Cloud Storage option

To deploy the database on Cloud Run follow these steps:

1. Create a bucket (you can also reuse an existing one) and, optionally, activate the versioning and set a lifecycle to
limit the versioning depth and cost
```Bash
# Create the bucket
gsutil mb gs://<BUCKET_NAME>

# Optional, activate the versioning
gsutil versioning set on gs://<BUCKET_NAME>

# Optional, set a lifecycle to limit versioning depth. Here keep 10 backup versions 
cat > lifecycle.json  << EOF
{
  "lifecycle": {
    "rule": [
    {
    "action": {"type": "Delete"},
    "condition": {
      "numNewerVersions": 10,
      "isLive": false
    }
  }
  ]
  }
}
EOF

gsutil lifecycle set lifecycle.json gs://<BUCKET_NAME>

```
2. (optional) Create a service account with the permission to read and write to the bucket
```Bash
# Create the service account
gcloud iam service-accounts create <SA_NAME>

# Get the SA email
gcloud iam service-accounts list --format="value(email)" | grep <SA_NAME>

# Grant the permission on the bucket
gsutil iam ch serviceAccount:<SA_EMAIL>:objectAdmin gs://<BUCKET_NAME>
```
3. Deploy the container to Cloud Run
```Bash
gcloud beta run deploy <SERVICE NAME> \
  --region=<YOUR REGION> \
  --image=us-central1-docker.pkg.dev/gblaquiere-dev/public/serverless-sql \
  --service-account=<SA_EMAIL> #optional. Must have the permission on the bucket \
  --allow-unauthenticated #optionnal. If not, proxy must use authenticated mode \
  --execution-environment gen2 \
  --max-instances=1 \
  --platform=managed \
  --memory=1024Mi \
  --use-http2 \
  --set-env-vars=BUCKET=<BUCKET_NAME>,ROOT_PASSWORD=<DB ROOT PASSWORD>   
```

**Parameters' explanation**

* **service-account**: *optional*. Must have the permission on the bucket 
* **allow-unauthenticated**: *optional*. If not set, proxy must use authenticated mode 
* **execution-environment**: The gen2 runtime isn't sandboxed and you won't have runtime warning because of that 
 sandbox. But also work on gen1 runtime
* **max-instances**: multi master isn't possible. Only 1 instance can be use at a time.
* **memory**: minimum memory to load the database engine and data in memory. Must be increase if the database
becomes bigger.
* **use-http2**: HTTP/2 protocol in bidirectional streaming is used to communicate between the proxy and the service
* **set-env-vars**: Minimal environment variable:
  * **BUCKET**: Name of the bucket to get and store the database data
  * **ROOT_PASSWORD**: Root password to connect to the database. The root password can be stored in 
  [Secret Manager](https://cloud.google.com/secret-manager). In that case, the `ROOT_PASSWORD` must be provided in 
  that format `sm://projects/<PROJECT_ID>/secrets/<SECRET_NAME>/versions/<VERSION>`. And the Cloud Run service account
  must have the access permission to the secret `roles/secretmanager.secretAccessor`.

Use the URL provided by the deployment in the proxy to connect it

## Cloud Filestore option

To deploy the database on Cloud Run follow these steps:

1. Create a Cloud Filestore storage in a region. Use the same region as your Cloud Run deployment for better 
performances.
```Bash
gcloud filestore instances create serverless-mysql \
 --tier=STANDARD \
 --zone=us-central1-a \
 --file-share=name=mysql_data,capacity=1TiB \
 --network=name="default"
```
2. Create a serverless VPC connector to bridge the serverless world where run Cloud Run and your VPC on which Cloud
Filestore is attached
```Bash
gcloud compute networks vpc-access connectors create serverless-mysql \
 --region us-central1 \
 --range "10.8.0.0/28"
```
3. Deploy the container (filestore version) to Cloud Run with the serverless VPC connector and the Cloud Filestore IP
```Bash
gcloud beta run deploy <SERVICE NAME>  \
  --region=<YOUR REGION> \
  --image=us-central1-docker.pkg.dev/gblaquiere-dev/public/serverless-sql-filestore \
  --service-account=<SA_EMAIL> #optional. Must have the permission on the bucket \
  --allow-unauthenticated #optionnal. If not, proxy must use authenticated mode \
  --execution-environment gen2 \
  --max-instances=1 \
  --platform=managed \
  --memory=512Mi \
  --use-http2 \
  --vpc-connector serverless-mysql \
  --set-env-vars=ROOT_PASSWORD=<DB ROOT PASSWORD>,FILESTORE_IP_ADDRESS=$(gcloud filestore instances describe serverless-mysql --format "value(networks.ipAddresses[0])" --zone=us-central1-a),FILE_SHARE_NAME=mysql_data   
```

**Parameters' explanation**

* **service-account**: *optional*. Must have the permission on the bucket
* **allow-unauthenticated**: *optional*. If not set, proxy must use authenticated mode
* **execution-environment**: The gen2 runtime isn't sandboxed and you won't have runtime warning because of that
  sandbox. Gen2 allows to mount network drive, like NFS. Doesn't work with gen1
* **max-instances**: multi master isn't possible. Only 1 instance can be use at a time.
* **memory**: minimum memory to load the database engine. Must be increase if the database becomes bigger.
* **use-http2**: HTTP/2 protocol in bidirectional streaming is used to communicate between the proxy and the service
* **set-env-vars**: Minimal environment variable:
  * **ROOT_PASSWORD**: Root password to connect to the database. The root password can be stored in
    [Secret Manager](https://cloud.google.com/secret-manager). In that case, the `ROOT_PASSWORD` must be provided in
    that format `sm://projects/<PROJECT_ID>/secrets/<SECRET_NAME>/versions/<VERSION>`. And the Cloud Run service account
    must have the access permission to the secret `roles/secretmanager.secretAccessor`.
  * **FILESTORE_IP_ADDRESS**: IP of the Cloud Filestore. *Here automatically recovered with a gcloud command*
  * **FILE_SHARE_NAME**: File share name define in Cloud Filestore. *You can add subpath if required.*
  
Use the URL provided by the deployment in the proxy to connect it

## Cloud Storage and Cloud Filestore: How to make the choice.

Cloud Storage is the cheapest solution but not the strongest. Indeed, in case of container crash (out of memory for 
insance), or in case of double instance (case where you deploy a new version with new parameters), you can have 
data loss. In addition, the cold start, the first start of the Cloud Run instance when no one are provisioned, is long 
(about 7s).

Cloud Filestore solves all that issues, but the 1Tb min capacity of Filestore create an expensive solution (about $200
per month), plus the serverless VPC connector (min $17 per month). That solution cost $217 per month, even if you don't
use it. 

# Use of the proxy

The proxy wrap the database TCP client connection in HTTP/2 protocol and call the serverless sql service running on 
Cloud Run (but can run elsewhere)

Download the version of the proxy for your environment (see "Quick Links")

## Proxy parameters

You can configure the proxy with different parameters
* **url**: Set the URL of the serverless database service to connect. If not set or set to empty string, the proxy 
exit gracefully. 
* **port**: The local port to listen. `3306` by default.
* **no-tls**: Deactivate the TLS support for HTTP/2 protocol (activate the clear text mode -> h2c). False by default, 
only for local tests

Example
```Bash
serverless-db-proxy --url=https://localhost:8080 --no-tls=true --port=4226
```

## Use proxy locally

To use it locally, download the binary according to your environment, and run it with proper parameters. The proxy
run in and wait database connection.

In another environment (your IDE, another shell), run your app or your database connexion on the `localhost:<port>`.

## Use proxy in container

If your environment allow you to start several container in the same time (Docker compose or Kubernetes Pods), you can
add that container in the configuration `us-central1-docker.pkg.dev/gblaquiere-dev/public/serverless-db-proxy`

If you use the proxy in your container, and you can't define several container in your deployment (like CLoud Run), you
have to run in background the proxy while you run your app in the container. **The runtime environment must contain
`/bin/bash` to run the startup script**

The proposed way is a simple wrapping into a startup shell. That way works well but not propagate the 
signals (TERM or INT for instance to stop the application.). 

*To propagate the signals, you can use [dumb-init](https://github.com/Yelp/dumb-init) as used in
[serverless-sql Dockerfile](https://github.com/guillaumeblaquiere/serverless-sql/tree/master/serverless-sql/Dockerfile)*

### Container parameters

In the `startup.sh` file that start the proxy and wrap the app execution, you can customize the proxy execution
parameters with environment variables

* **URL**: *Equivalent to `--url` parameter.* Set the URL of the serverless database service to connect. If not set or 
set to empty string, the proxy exit gracefully.
* **PORT**: *Equivalent to `--port` parameter.* The local port to listen. `3306` by default.
* **NO_TLS**: *Equivalent to `--no-tls` parameter.* Deactivate the TLS support for HTTP/2 protocol (activate the clear
text mode -> h2c). False by default, only for local tests

### Container layer mode

To import the files directly from the official `serverless-db-proxy` container, you have to
* Add Serverless db proxy container as layer in your`Dockerfile`, *for instance at the beginning*
* Copy the `/serverless-db-proxy` binary from the proxy layer to the root path in your final layer
* Copy the `/startup.sh` script from the proxy layer to the root path in your final layer
* Define the `/startup.sh` as the entry point of the container
* Define your app startup command as `CMD` in your final layer

Similar to that

```Dockerfile
# Import the Serverless db proxy as proxy layer
FROM us-central1-docker.pkg.dev/gblaquiere-dev/public/serverless-db-proxy AS proxy

# Build your container as usual
FROM ...
...

# At the end of your build
# Add the startup script
COPY --from=proxy /startup.sh / 
# Add the binary proxy
COPY --from=proxy /serverless-db-proxy / 

# Define startup.sh as entrypoint
ENTRYPOINT ["/startup.sh"] 

# Set your app entrypoint as CMD
CMD ["/my-app"] 
```

### File download

To download the files directly from the official `serverless-db-proxy` Cloud Storage, you have to
* Download `/serverless-db-proxy` binary from Cloud Storage to the root path in your final layer
* Copy the `/startup.sh` script from Cloud Storage to the root path in your final layer
* Define the `/startup.sh` as the entry point of the container
* Define your app startup command as `CMD` in your final layer

Similar to that

```Dockerfile
# Build your container as usual
FROM ...
...

# At the end of your build
# Download the startup script 
RUN wget https://storage.googleapis.com/serverless-db-proxy/startup.sh -P / && chmod +x /startup.sh
# Download the binary proxy
RUN wget https://storage.googleapis.com/serverless-db-proxy/linux64/serverless-db-proxy -P / && chmod +x /serverless-db-proxy

# Define startup.sh as entrypoint
ENTRYPOINT ["/startup.sh"] 

# Set your app entrypoint as CMD
CMD ["/my-app"] 
```

# License

This library is licensed under Apache 2.0. Full license text is available in
[LICENSE](https://github.com/guillaumeblaquiere/serverless-sql/tree/master/LICENSE).
