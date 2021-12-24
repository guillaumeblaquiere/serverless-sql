# Overview

That proof of concept use Cloud Run to deploy a SQL database on demand. That solution prevent to use Cloud SQL for 
development database or for non critical and low usage production database.

TODO -> link to Medium Article

# Technical overview

There is 2 parts at that solution

* the serverless sql container
* the serverless database proxy

## Serverless sql container

That container is composed of 2 parts

* The database engine itself that run as background process in the container
* The HTTP interface that serve the traffic.

The process is the following
1. The database engine starts when the container starts. The data are loaded from Cloud Storage, gunzip and untar
2. The database engine runs in the container and use the data loaded in the memory of the container
3. When the container is stoped (signals TERM or INT received), the data are tar and gzip and sent to Cloud Storage

The use of the HTTP endpoint is described in the "Communication between the 2 parties" section

## Serverless database proxy

The proxy listen the TCP communication and act as proxy to forward them to the serverless sql Cloud Run service.

## Communication between the 2 parties

Cloud Run only accept HTTP connection and disallow TCP connection. That's why, the TCP communication with the database 
must be wrapped in HTTP protocol.

For that, the proxy and the Serverless SQL container open HTTP/2 connection and use bidirectional streaming to 
exchange data. The proxy initiate the connection.

# Deployment of the database


# Use of the proxy

## Use proxy locally

## Use proxy in container

# License

This library is licensed under Apache 2.0. Full license text is available in
[LICENSE](https://github.com/guillaumeblaquiere/serverless-sql/tree/master/LICENSE).