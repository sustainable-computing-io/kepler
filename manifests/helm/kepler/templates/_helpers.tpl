{{/*
Expand the name of the chart.
*/}}
{{- define "kepler.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "kepler.fullname" -}}
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
{{- define "kepler.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "kepler.labels" -}}
helm.sh/chart: {{ include "kepler.chart" . }}
{{ include "kepler.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: kepler
{{- with .Values.labels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "kepler.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kepler.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "kepler.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "kepler.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Name of the metrics port, shared by the container, the Service and the ServiceMonitor.
Falls back to http when the value is missing, as it is when upgrading with --reuse-values from a chart that predates it.
*/}}
{{- define "kepler.portName" -}}
{{- default "http" (dig "port" "name" "" .Values.daemonset) -}}
{{- end }}

{{/*
Render a probe, defaulting httpGet.port to the metrics port name when it is not set
*/}}
{{- define "kepler.probe" -}}
{{- $probe := deepCopy .probe -}}
{{- if and $probe.httpGet (not $probe.httpGet.port) -}}
{{- $_ := set $probe.httpGet "port" .portName -}}
{{- end -}}
{{- toYaml $probe -}}
{{- end }}

{{/*
Create the namespace to use
*/}}
{{- define "kepler.namespace" -}}
{{- if .Values.namespace.create }}
{{- default (include "kepler.name" .) .Values.namespace.name }}
{{- else }}
{{- .Release.Namespace }}
{{- end }}
{{- end }}
