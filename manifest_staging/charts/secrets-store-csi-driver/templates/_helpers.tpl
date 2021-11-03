{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "sscd.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "sscd.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Standard labels for helm resources
*/}}
{{- define "sscd.labels" }}
app.kubernetes.io/managed-by: "{{ .Release.Service }}"
app.kubernetes.io/part-of: "{{ template "sscd.name" . }}"
app.kubernetes.io/component: csi-driver
app.kubernetes.io/version: "{{ .Chart.AppVersion }}"
{{- include "sscd.selectorLabels" . }}
helm.sh/chart: "{{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}"
{{- if .Values.customLabels }}
{{ toYaml .Values.customLabels }}
{{- end }}
{{- end -}}

{{/*
Selector labels
*/}}
{{- define "sscd.selectorLabels" }}
app.kubernetes.io/name: {{ include "sscd.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app: {{ template "sscd.name" . }}
{{- end }}

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
