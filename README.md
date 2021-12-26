# Overview

That proof of concept use Cloud Run to deploy a SQL database on demand. That solution prevent to use Cloud SQL for 
development database or for non-critical and low usage production database.

TODO -> link to Medium Article

# Technical overview

There is 2 parts at that solution

* the serverless sql container
* the serverless database proxy

# Quick links

* Packaged container to deploy on Cloud Run (or elsewhere): 
  * `us-central1-docker.pkg.dev/gblaquiere-dev/public/serverless-sql`
* Compiled serverless db proxy
  * [Windows/amd64](https://storage.cloud.google.com/serverless-db-proxy/master/win64/serverless-db-proxy.exe)
  * [Linux/amd64](https://storage.cloud.google.com/serverless-db-proxy/master/linux64/serverless-db-proxy)
  * [Darwin/amd64](https://storage.cloud.google.com/serverless-db-proxy/master/darwin64/serverless-db-proxy)

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

## Communication between the 2 parties

Cloud Run only accept HTTP connection and disallow TCP connection. That's why, the TCP communication with the database 
must be wrapped in HTTP protocol.

For that, the proxy and the Serverless SQL container open HTTP/2 connection and use bidirectional streaming to 
exchange data. The proxy initiate the connection.

# Deployment of the database

To deploy the database on Cloud Run follow these steps:

1. Create a bucket (you can also reuse an existing one)
```
gsutil mb gs://<BUCKET_NAME>
```
2. (optional) Create a service account with the permission to read and write to the bucket
```
# Create the service account
gcloud iam service-accounts create <SA_NAME>

# Get the SA email
gcloud iam service-accounts list --format="value(email)" | grep <SA_NAME>

# Grant the permission on the bucket
gsutil iam ch serviceAccount:<SA_EMAIL>:objectAdmin gs://<BUCKET_NAME>
```
3. Deploy the container to Cloud Run
```
gcloud beta run deploy <SERVICE NAME> --platform=manager \
  --region=<YOUR REGION> \
  --image=us-central1-docker.pkg.dev/gblaquiere-dev/public/serverless-sql \
  --service-account=<SA_EMAIL> #optional. Must have the permission on the bucket \
  --allow-unauthenticated #optionnal. If not, proxy must use authenticated mode \
  --execution-environment gen2 \
  --max-instances=1 \
  --memory 1024Mi \
  --use-http2 \
  --set-env-vars=BUCKET=<BUCKET_NAME>,ROOT_PASSWORD=<DB ROOT PASSWORD>   
```

**Parameters' explanation**

* **service-account**: *optional*. Must have the permission on the bucket 
* **allow-unauthenticated**: *optional*. If not set, proxy must use authenticated mode 
* **execution-environment**: The gen2 runtime isn't sandboxed and you won't have runtime warning because of that 
 sandbox. But also work on gen1 runtime
* **max-instances=1**: multi master isn't possible. Only 1 instance can be use at a time.
* **memory 1024Mi**: minimum memory to load the database engine and data in memory. Must be increase if the database
becomes bigger.
* **use-http2**: HTTP/2 protocol in bidirectional streaming is used to communicate between the proxy and the service
* **set-env-vars**: Minimal environment variable:
  * **BUCKET**: Name of the bucket to get and store the database data
  * **ROOT_PASSWORD**: Root password to connect to the database.

Use the URL provided by the deployment in the proxy to connect it

# Use of the proxy

The proxy wrap the database TCP client connection in HTTP/2 protocol and call the serverless sql service running on 
Cloud Run (but can run elsewhere)

Download the version of the proxy for your environment (see "Quick Links")

## Use proxy locally

## Use proxy in container

# License

This library is licensed under Apache 2.0. Full license text is available in
[LICENSE](https://github.com/guillaumeblaquiere/serverless-sql/tree/master/LICENSE).