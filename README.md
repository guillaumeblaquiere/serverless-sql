# Overview
This repository present the different way to contact an Oracle Database with GCP serverless Product.

AppEngine standard and flex, Cloud Run and function are used. Except for AppEngine8, the 4 products are usable with the same source code.

Think to customize the configuration files with your values.

# Deployment and tests

## Java
### AppEngine Java8
AppEngine Java 8 requires specific dependencies and code structure which is not compliant with other GCP product.

Java directory is compliant AppEngine Standard Java11, AppEngine flex, Cloud Function, Cloud Run

```bash
# Go to the directory
cd appEngine8

# Update the file in src/main/webapp/WEB-INF/appengine-web.xml with your environment variables

# Run Mvn command. Maven 3.5 or above must be installed
mvn clean package appengine:deploy

# Test your appEngine
curl $(gcloud app browse -s java8-serverless-oracle \
    --no-launch-browser)
```

### AppEngine Java11
It's not a real Java11 version. It's a Java 8 but fully compliant with Java11 environment.
The Java8 is kept for the test of Alpha version of Cloud Function Java.

Fat Jar mode is used to embed the Oracle Jar in the deployment. Only Standard deployment is perform. It's enough and cheaper

In the `pom.xml` files, change the `PROJECT_ID` value with your project id. In the `src/main/appengine` update the `app.yaml` with your env vars values
```bash
# Go to the directory
cd java

# Run Mvn command. Maven 3.5 or above must be installed
mvn clean package appengine:deploy

# Test your appEngine
curl $(gcloud app browse -s java11-serverless-oracle \
    --no-launch-browser)   
```

### AppEngine flexible Java11
For AppEngine flex with custom runtime, `Dockerfile` and `cloudbuild.yaml` file can't be in the same directory.
That's why, `googlecloudbuild.yaml` exists instead of the regular name

Update the `app-flexible.yaml` with your env vars values
```bash
# Go to the directory
cd java

# Run Mvn command. Maven 3.5 or above must be installed
gcloud app deploy app-flexible.yaml

# Test your appEngine
curl $(gcloud app browse -s java11-serverless-oracle-flex \
    --no-launch-browser)
```

## Cloud Function Java
The Java8 is kept for the test of Alpha version of Cloud Function Java.
Fat Jar mode is used to embed the Oracle Jar in the deployment

```bash
# Go to the directory
cd java

# Deploy the alpha function
# Change the env vars by your values
gcloud alpha functions deploy oracle-serverless --trigger-http --region us-central1 \
   --runtime java11 --allow-unauthenticated \
   --entry-point dev.gblaquiere.serverlessoracle.java.function.OracleConnection \
   --set-env-vars ORACLE_IP=<YOUR IP>,ORACLE_SCHEMA=<YOUR SCHEMA>,\
ORACLE_USER=<YOUR USER>,ORACLE_PASSWORD=<YOUR PASSWORD>

# Test your function 
gcloud functions call oracle-serverless
```

## Cloud Run Java

In the `pom.xml` files, change the `PROJECT_ID` value with your project id

### With Cloud Build
For AppEngine flex with custom runtime, `Dockerfile` and `cloudbuild.yaml` file can't be in the same directory.
That's why, `googlecloudbuild.yaml` exists instead of the regular name

```bash
# Go to the directory
cd java

# Run the build
gcloud builds submit --config googlecloudbuild.yaml

# Deploy on Cloud run 
# Change <PROJECT_ID> by your project ID. Change the env vars by your values
gcloud run deploy java-serverless-oracle --region us-central1 --platform managed \
    --allow-unauthenticated --image gcr.io/<PROJECT_ID>/java-serverless-oracle \
    --set-env-vars ORACLE_IP=<YOUR IP>,ORACLE_SCHEMA=<YOUR SCHEMA>,\
ORACLE_USER=<YOUR USER>,ORACLE_PASSWORD=<YOUR PASSWORD>

# Test your deployment
curl $(gcloud run services describe java-serverless-oracle --region us-central1 \
    --format "value(status.address.hostname)" --platform managed)
```

### With JIB
```bash
# Go to the directory
cd java

# Run the build
mvn clean compile jib:build

# Deploy on Cloud run 
# Change <PROJECT_ID> by your project ID. Change the env vars by your values
gcloud run deploy java-serverless-oracle --region us-central1 --platform managed \
    --allow-unauthenticated --image gcr.io/<PROJECT_ID>/java-serverless-oracle-jib \
    --set-env-vars ORACLE_IP=<YOUR IP>,ORACLE_SCHEMA=<YOUR SCHEMA>,\
ORACLE_USER=<YOUR USER>,ORACLE_PASSWORD=<YOUR PASSWORD>

# Test your deployment
curl $(gcloud run services describe java-serverless-oracle-jib --region us-central1 \
    --format "value(status.address.hostname)" --platform managed)
```

## Go
### Function
Not applicable (no instant client)

However, if you want to tests

```bash
# Go to the directory
cd go

# copy the dependencies close to the function file
cp go.mod function/

# Deploy the alpha function
# Change the env vars by your values
gcloud beta functions deploy go-oracle-serverless --trigger-http --region us-central1 \
   --runtime go112 --source function --allow-unauthenticated \
   --entry-point OracleConnection \
   --set-env-vars ORACLE_IP=<YOUR IP>,ORACLE_SCHEMA=<YOUR SCHEMA>,\
ORACLE_USER=<YOUR USER>,ORACLE_PASSWORD=<YOUR PASSWORD>

# Clean up the function directory
rm function/go.mod

# Test your function 
gcloud functions call go-oracle-serverless
```

### AppEngine Standard
Not applicable (no instant client)

Try this `gcloud app deploy app-standard.yaml` for validating that is doesn't work. Update the `app-standard.yaml` with your env vars values

### AppEngine Flexible
For AppEngine flex with custom runtime, `Dockerfile` and `cloudbuild.yaml` file can't be in the same directory.
That's why, `googlecloudbuild.yaml` exists instead of the regular name

Update the `app-flexible.yaml` with your env vars values
```bash
# Go to the directory
cd go

# Run Mvn command. Maven 3.5 or above must be installed
gcloud app deploy app-flexible.yaml

# Test your appEngine
curl $(gcloud app browse -s go-serverless-oracle-flex \
    --no-launch-browser)
```

### Cloud Run
For AppEngine flex with custom runtime, `Dockerfile` and `cloudbuild.yaml` file can't be in the same directory.
That's why, `googlecloudbuild.yaml` exists instead of the regular name

```bash
# Go to the directory
cd go

# Run the build
gcloud builds submit --config googlecloudbuild.yaml

# Deploy on Cloud run 
# Change <PROJECT_ID> by your project ID. Change the env vars by your values
gcloud run deploy go-serverless-oracle --region us-central1 --platform managed \
    --allow-unauthenticated --image gcr.io/<PROJECT_ID>/go-serverless-oracle \
    --set-env-vars ORACLE_IP=<YOUR IP>,ORACLE_SCHEMA=<YOUR SCHEMA>,\
ORACLE_USER=<YOUR USER>,ORACLE_PASSWORD=<YOUR PASSWORD>

# Test your deployment
curl $(gcloud run services describe go-serverless-oracle --region us-central1 \
    --format "value(status.address.hostname)" --platform managed)
```

## NodeJS
### Function
Not applicable (no instant client)

However, if you want to tests

```bash
# Go to the directory
cd nodejs

# copy the dependencies close to the function file
cp package.json function/

# Deploy the alpha function
# Change the env vars by your values
gcloud beta functions deploy nodejs-oracle-serverless --trigger-http --region us-central1 \
   --runtime nodejs10 --source function --allow-unauthenticated \
   --entry-point oracleConnection \
   --set-env-vars ORACLE_IP=<YOUR IP>,ORACLE_SCHEMA=<YOUR SCHEMA>,\
ORACLE_USER=<YOUR USER>,ORACLE_PASSWORD=<YOUR PASSWORD>

# Clean up the function directory
rm function/package.json

# Test your function 
gcloud functions call nodejs-oracle-serverless
```

### AppEngine Standard
Not applicable (no instant client)

Try this `gcloud app deploy app-standard.yaml` for validating that is doesn't work. Update the `app-standard.yaml` with your env vars values


### AppEngine Flexible
For AppEngine flex with custom runtime, `Dockerfile` and `cloudbuild.yaml` file can't be in the same directory.
That's why, `googlecloudbuild.yaml` exists instead of the regular name

Update the `app-flexible.yaml` with your env vars values

```bash
# Go to the directory
cd nodejs

# Run Mvn command. Maven 3.5 or above must be installed
gcloud app deploy app-flexible.yaml

# Test your appEngine
curl $(gcloud app browse -s nodejs-serverless-oracle-flex \
    --no-launch-browser)
```

### Cloud Run
For AppEngine flex with custom runtime, `Dockerfile` and `cloudbuild.yaml` file can't be in the same directory.
That's why, `googlecloudbuild.yaml` exists instead of the regular name

```bash
# Go to the directory
cd nodejs

# Run the build
gcloud builds submit --config googlecloudbuild.yaml

# Deploy on Cloud run 
# Change <PROJECT_ID> by your project ID. Change the env vars by your values
gcloud run deploy nodejs-serverless-oracle --region us-central1 --platform managed \
    --allow-unauthenticated --image gcr.io/<PROJECT_ID>/nodejs-serverless-oracle \
    --set-env-vars ORACLE_IP=<YOUR IP>,ORACLE_SCHEMA=<YOUR SCHEMA>,\
ORACLE_USER=<YOUR USER>,ORACLE_PASSWORD=<YOUR PASSWORD>

# Test your deployment
curl $(gcloud run services describe nodejs-serverless-oracle --region us-central1 \
    --format "value(status.address.hostname)" --platform managed)
```

## Python
### Function
Not applicable (no instant client)

However, if you want to tests

```bash
# Go to the directory
cd python

# copy the dependencies close to the function file
cp requirements.txt function/

# Deploy the alpha function
# Change the env vars by your values
gcloud functions deploy python-oracle-serverless --trigger-http --region us-central1 \
   --runtime python37 --source function --allow-unauthenticated \
   --entry-point oracle_connection \
   --set-env-vars ORACLE_IP=<YOUR IP>,ORACLE_SCHEMA=<YOUR SCHEMA>,\
ORACLE_USER=<YOUR USER>,ORACLE_PASSWORD=<YOUR PASSWORD>

# Clean up the function directory
rm function/requirements.txt

# Test your function 
gcloud functions call python-oracle-serverless
```


### AppEngine Standard
Not applicable (no instant client)

Try this `gcloud app deploy app-standard.yaml` for validating that is doesn't work. Update the `app-standard.yaml` with your env vars values

### AppEngine Flexible
For AppEngine flex with custom runtime, `Dockerfile` and `cloudbuild.yaml` file can't be in the same directory.
That's why, `googlecloudbuild.yaml` exists instead of the regular name

Update the `app-flexible.yaml` with your env vars values

```bash
# Go to the directory
cd python

# Run Mvn command. Maven 3.5 or above must be installed
gcloud app deploy app-flexible.yaml

# Test your appEngine
curl $(gcloud app browse -s python-serverless-oracle-flex \
    --no-launch-browser)
```

### Cloud Run
For AppEngine flex with custom runtime, `Dockerfile` and `cloudbuild.yaml` file can't be in the same directory.
That's why, `googlecloudbuild.yaml` exists instead of the regular name

```bash
# Go to the directory
cd python

# Run the build
gcloud builds submit --config googlecloudbuild.yaml

# Deploy on Cloud run 
# Change <PROJECT_ID> by your project ID. Change the env vars by your values
gcloud run deploy python-serverless-oracle --region us-central1 --platform managed \
    --allow-unauthenticated --image gcr.io/<PROJECT_ID>/python-serverless-oracle \
    --set-env-vars ORACLE_IP=<YOUR IP>,ORACLE_SCHEMA=<YOUR SCHEMA>,\
ORACLE_USER=<YOUR USER>,ORACLE_PASSWORD=<YOUR PASSWORD>

# Test your deployment
curl $(gcloud run services describe python-serverless-oracle --region us-central1 \
    --format "value(status.address.hostname)" --platform managed)
```

# License

This library is licensed under Apache 2.0. Full license text is available in
[LICENSE](https://github.com/guillaumeblaquiere/serverless-oracle/tree/master/LICENSE).