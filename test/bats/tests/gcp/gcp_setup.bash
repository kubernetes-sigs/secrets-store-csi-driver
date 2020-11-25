#!/bin/bash

gcp_project=${GCP_PROJECT_NAME:-"gcp-e2e-test2"}
GCP_SA_JSON=${GCP_SA_JSON:-""}
secret_name=test-secret-a
service_account=${SERVICE_ACCOUNT:-"gcp-test"}

export RESOURCE_NAME=projects/$gcp_project/secrets/$secret_name/versions/latest

gcloud config set project $gcp_project


#To-do check if the SA already exsits 
gcloud iam service-accounts create gcp-test --display-name "GCP e2e test service account"

#To-do check if the key already exsits by retrieving it 
if [ "$GCP_SA_JSON" = "" ]; then
  
  GCP_SA_JSON=$(gcloud iam service-accounts keys create --iam-account $service_account@$gcp_project.iam.gserviceaccount.com gcpsajson.json)

  gcloud services enable secretmanager.googleapis.com

#To-do check if the secret already exsits
  printf "hunter2" | gcloud secrets create $secret_name --replication-policy="automatic" --data-file="-"

  gcloud secrets add-iam-policy-binding $secret_name --member=serviceAccount:$service_account@$gcp_project.iam.gserviceaccount.com --role=roles/secretmanager.secretAccessor

  
  if [ $? -ne 0 ]; then
      echo "Error: Cannot export GCP Service Account (GCP_SA_JSON)"
      return 1
  fi

  echo $GCP_SA_JSON
  export GCP_SA_JSON=$GCP_SA_JSON

fi
