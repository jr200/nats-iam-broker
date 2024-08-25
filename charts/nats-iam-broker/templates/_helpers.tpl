{{/*
Expand the name of the chart.
*/}}
{{- define "nats-iam-broker.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "nats-iam-broker.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "nats-iam-broker.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "nats-iam-broker.labels" -}}
helm.sh/chart: {{ include "nats-iam-broker.chart" . }}
{{ include "nats-iam-broker.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "nats-iam-broker.selectorLabels" -}}
app.kubernetes.io/name: {{ include "nats-iam-broker.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "nats-iam-broker.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "nats-iam-broker.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
------------------------------------------------------------------------------
Additional helper functions
------------------------------------------------------------------------------
*/}}
{{- define "secretValue" -}}
{{- $type := index . 0 -}}
{{- $x := index . 1 -}}
{{ if eq $type "env" -}}
<<< env ${{ $x }} >>>
{{- else if eq $type "file" -}}
<<< readFile {{ $x | quote }} >>>
{{- else -}}
{{ $x }}
{{- end }}
{{- end }}

{{- define "getImageTag" -}}
  {{- $semverRegex := `^[0-9]+\.[0-9]+\.[0-9]+$` -}}
  {{- $imageTag := .Values.image.tag | default .Chart.AppVersion | trimPrefix "v" -}}
  {{- if regexMatch $semverRegex $imageTag -}}
    {{- $imageTag -}}
  {{- else -}}
    {{- .Chart.AppVersion | trimPrefix "v" -}}
  {{- end -}}
{{- end -}}