# shcv
shcv stands for Sync Helm Chart Values - it traverses your Helm chart manifest files, compares their use of {{ .Values.* }} to values.yaml and adds any missing parameters.
