{{/*
Expand the name of the chart.
*/}}
{{- define "encodeswarmr.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "encodeswarmr.fullname" -}}
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
Create chart label.
*/}}
{{- define "encodeswarmr.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "encodeswarmr.labels" -}}
helm.sh/chart: {{ include "encodeswarmr.chart" . }}
{{ include "encodeswarmr.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "encodeswarmr.selectorLabels" -}}
app.kubernetes.io/name: {{ include "encodeswarmr.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Controller selector labels.
*/}}
{{- define "encodeswarmr.controllerSelectorLabels" -}}
app.kubernetes.io/name: {{ include "encodeswarmr.name" . }}-controller
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: controller
{{- end }}

{{/*
Agent selector labels.
*/}}
{{- define "encodeswarmr.agentSelectorLabels" -}}
app.kubernetes.io/name: {{ include "encodeswarmr.name" . }}-agent
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: agent
{{- end }}

{{/*
ServiceAccount name.
*/}}
{{- define "encodeswarmr.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "encodeswarmr.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Controller image tag (falls back to appVersion).
*/}}
{{- define "encodeswarmr.controllerImage" -}}
{{ .Values.image.controller.repository }}:{{ .Values.image.controller.tag | default .Chart.AppVersion }}
{{- end }}

{{/*
Agent image tag (falls back to appVersion).
*/}}
{{- define "encodeswarmr.agentImage" -}}
{{ .Values.image.agent.repository }}:{{ .Values.image.agent.tag | default .Chart.AppVersion }}
{{- end }}
