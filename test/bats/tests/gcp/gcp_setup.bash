#!/bin/bash

gcp_project=${GCP_PROJECT_NAME:-"gcp-e2e-test2"}
sa_account_path="test/bats/tests/gcp/gcpsajson.json"
secret_name="test-secret-a"
service_account=${SERVICE_ACCOUNT:-"gcp-test"}

gcloud config set project $gcp_project


#Check if the key already exsits by retrieving it 

if [ ! -f "$sa_account_path" ]; then
  
    gcloud iam service-accounts keys create --iam-account $service_account@$gcp_project.iam.gserviceaccount.com $sa_account_path

    if [ $? -ne 0 ]; then
        echo "Error: Cannot export GCP Service Account (GCP_SA_JSON)"
        return 1
    fi

    gcloud services enable secretmanager.googleapis.com

    printf "hunter2" | gcloud secrets create $secret_name --replication-policy="automatic" --data-file="-"

    gcloud secrets add-iam-policy-binding $secret_name --member=serviceAccount:$service_account@$gcp_project.iam.gserviceaccount.com --role=roles/secretmanager.secretAccessor

    if [ $? -ne 0 ]; then
        echo "Error: Cannot create secret for the service account"
        return 1
    fi
fi

 export GCP_SA_JSON=$sa_account_path
