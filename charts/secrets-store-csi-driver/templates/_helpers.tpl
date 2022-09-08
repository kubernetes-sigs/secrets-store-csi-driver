{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "sscd.name" -}}
{{/* Add non-global support for backwards compatability.
Presedence is global > non-global > .Chart.Name
*/}}
{{- default (default .Chart.Name  .Values.nameOverride) .Values.global.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "sscd.fullname" -}}
{{- if .Values.global.fullnameOverride -}}
{{- .Values.global.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- /* Add non-global check for backwards compatability */}}
{{- else if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default (default .Chart.Name .Values.nameOverride) .Values.global.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Standardize the upgrade-crds hook name by ensuring total truncation to 63 chars.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "sscd.fullname.upgradeCRDs" -}}
{{ include "sscd.fullname.suffixAdd" (list $ "upgrade-crds") }}
{{- end -}}

{{/*
Standardize the keep-crds hook name by ensuring total truncation to 63 chars.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "sscd.fullname.keepCRDs" -}}
{{ include "sscd.fullname.suffixAdd" (list $ "keep-crds") }}
{{- end -}}

{{/*
General-purpose function to truncate the sscd fullname to fit the provided suffix
and remain within 63 chars.
Expects a list containing 2 parameters -
0: The root context
1: The suffix that will be added, excluding leading dash
*/}}
{{- define "sscd.fullname.suffixAdd" -}}
{{- /* Unfurl parameters */}}
{{- $root := index . 0 }}
{{- $suffix := printf "-%s" (index . 1) }}
{{- /* Calculate truncation length */}}
{{- $truncLen := sub  63 (len $suffix) }}
{{- /* Truncate sscd.fullname based on suffix length length */}}
{{- $truncated := (include "sscd.fullname" $root) | trunc (int $truncLen) | trimSuffix "-" }}
{{- /* Output final string */}}
{{- printf "%s%s" $truncated $suffix }}
{{- end -}}

{{/*
Standard labels for helm resources
*/}}
{{- define "sscd.labels" -}}
app.kubernetes.io/instance: "{{ .Release.Name }}"
app.kubernetes.io/managed-by: "{{ .Release.Service }}"
app.kubernetes.io/name: "{{ template "sscd.name" . }}"
app.kubernetes.io/version: "{{ .Chart.AppVersion }}"
app: {{ template "sscd.name" . }}
helm.sh/chart: "{{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}"
{{- if .Values.global.commonLabels}}
{{ toYaml .Values.global.commonLabels }}
{{- else if .Values.commonLabels}}
{{ toYaml .Values.commonLabels }}
{{- end }}
{{- end -}}

{{- define "sscd-psp.fullname" -}}
{{- printf "%s-psp" (include "sscd.name" .) -}}
{{- end }}

{{/*
Return the appropriate apiVersion for CSIDriver.
*/}}
{{- define "csidriver.apiVersion" -}}
{{- if semverCompare ">=1.18-0" .Capabilities.KubeVersion.Version }}
{{- print "storage.k8s.io/v1" -}}
{{- else -}}
{{- print "storage.k8s.io/v1beta1" -}}
{{- end -}}
{{- end -}}
