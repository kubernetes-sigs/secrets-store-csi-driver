#!/bin/bash

gcp_project=${GCP_PROJECT_NAME:-"gcp-e2e-test"}
GCP_SA_JSON=${GCP_SA_JSON:-""}
secret_name=test-secret-a

export RESOURCE_NAME=projects/$gcp_project/secrets/$secret_name/versions/latest

gcloud config set project $gcp_project


if [ "$GCP_SA_JSON" = "" ]; then
  ##gcloud iam service-accounts create gcp-test --display-name "GCP e2e test service account"
  GCP_SA_JSON=$(gcloud iam service-accounts keys create gcpsajson.json --iam-account gcp-test@$gcp_project.iam.gserviceaccount.com)
fi

if [ $? -ne 0 ]; then
    echo "Error: Cannot export GCP Service Account (GCP_SA_JSON)"
    return 1
fi

printf "hunter2" gcloud secrets create $secret_name --replication-policy="automatic" --data-file="-"

gcloud secrets add-iam-policy-binding $secret_name --member=serviceAccount:gcp-test@$gcp_project.iam.gserviceaccount.com --role=roles/secretmanager.secretAccessor

export GCP_SA_JSON=$GCP_SA_JSON
