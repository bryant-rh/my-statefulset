#!/bin/bash

# 设置变量
CHART_NAME="mystatefulset"
CHART_VERSION="0.1.0"
CHART_DIR="deploy/helm/${CHART_NAME}"
VERSION=$(cat .version)
COMMIT_SHA=$(git rev-parse --short HEAD)
# Image URL to use all building/pushing image targets
#IMG ?= controller:latest
HELM_IMG_REPO="bryantrh/mystatefulset-controller"
HELM_IMG_TAG="${VERSION}-${COMMIT_SHA}"

# 从 Makefile 获取 KUSTOMIZE 路径
KUSTOMIZE=$(pwd)/bin/kustomize

# 检查 kustomize 是否存在
if [ ! -f "${KUSTOMIZE}" ]; then
    echo "Error: kustomize not found at ${KUSTOMIZE}"
    echo "Please run 'make kustomize' first"
    exit 1
fi

echo "Using kustomize at: ${KUSTOMIZE}"

# 创建 Helm chart 目录结构
mkdir -p ${CHART_DIR}/templates
mkdir -p ${CHART_DIR}/crds

# 生成 Chart.yaml
cat >${CHART_DIR}/Chart.yaml <<EOF
apiVersion: v2
name: ${CHART_NAME}
description: A Helm chart for MyStatefulset Controller
type: application
version: ${CHART_VERSION}
appVersion: "${CHART_VERSION}"
EOF

# 生成默认的 values.yaml
cat >${CHART_DIR}/values.yaml <<EOF
# Default values for ${CHART_NAME}
nameOverride: ""
fullnameOverride: ""

replicaCount: 1

image:
  repository: ${HELM_IMG_REPO}
  tag: ${HELM_IMG_TAG}
  pullPolicy: IfNotPresent

imagePullSecrets: []
nameOverride: ""
fullnameOverride: ""

serviceAccount:
  create: true
  annotations: {}
  name: ""

podAnnotations: {}
podSecurityContext: {}

securityContext: {}

resources: {}

nodeSelector: {}

tolerations: []

affinity: {}

# Webhook configurations
webhook:
  enabled: true
  certManager:
    enabled: true
    
# Controller configurations
controller:
  manager:
    args:
      - --leader-elect
      - --health-probe-bind-address=:8081
      - --metrics-bind-address=127.0.0.1:8080
    env:
      - name: ENABLE_WEBHOOKS
        value: "true"
EOF

# 生成 CRD
echo "Generating CRDs..."
mkdir -p ${CHART_DIR}/crds
cp config/crd/bases/* ${CHART_DIR}/crds/

# 生成 templates
echo "Generating templates..."
"${KUSTOMIZE}" build config/default >all.yaml

# 使用 yq 分割不同的资源类型
echo "Splitting resources..."

# Deployment
yq eval 'select(.kind == "Deployment")' all.yaml >${CHART_DIR}/templates/deployment.yaml

# Service
yq eval 'select(.kind == "Service")' all.yaml >${CHART_DIR}/templates/service.yaml

# Webhooks
yq eval 'select(.kind == "ValidatingWebhookConfiguration" or .kind == "MutatingWebhookConfiguration")' all.yaml >${CHART_DIR}/templates/webhook.yaml

# RBAC
yq eval 'select(.kind == "ServiceAccount" or .kind == "Role" or .kind == "RoleBinding" or .kind == "ClusterRole" or .kind == "ClusterRoleBinding")' all.yaml >${CHART_DIR}/templates/rbac.yaml

# Certificate
yq eval 'select(.kind == "Certificate" or .kind == "Issuer")' all.yaml >${CHART_DIR}/templates/certificate.yaml

# 添加 Helm 模板语法
echo "Adding Helm template syntax..."

# 创建临时文件来存储替换后的内容
if [[ "$OSTYPE" == "darwin"* ]]; then
    # MacOS
    echo "Detected MacOS, using compatible sed syntax..."

    # 替换 namespace
    find ${CHART_DIR}/templates -type f -exec sed -i '' 's/namespace: mystatefulset-system/namespace: {{ .Release.Namespace }}/g' {} \;

    # 替换 image
    find ${CHART_DIR}/templates -type f -exec sed -i '' 's|image: bryantrh/mystatefulset-controller:.*|image: {{ .Values.image.repository }}:{{ .Values.image.tag }}|g' {} \;

    # 添加 imagePullPolicy
    find ${CHART_DIR}/templates -type f -exec sed -i '' '/image: {{ .Values.image.repository }}:{{ .Values.image.tag }}/a\
          imagePullPolicy: {{ .Values.image.pullPolicy }}' {} \;
else
    # Linux
    echo "Detected Linux, using standard sed syntax..."

    # 替换 namespace
    find ${CHART_DIR}/templates -type f -exec sed -i 's/namespace: mystatefulset-system/namespace: {{ .Release.Namespace }}/g' {} \;

    # 替换 image
    find ${CHART_DIR}/templates -type f -exec sed -i 's|image: bryantrh/mystatefulset-controller:.*|image: {{ .Values.image.repository }}:{{ .Values.image.tag }}|g' {} \;

    # 添加 imagePullPolicy
    find ${CHART_DIR}/templates -type f -exec sed -i '/image: {{ .Values.image.repository }}:{{ .Values.image.tag }}/a\          imagePullPolicy: {{ .Values.image.pullPolicy }}' {} \;
fi

# 手动修改 deployment.yaml 中的 resources 部分
cat >${CHART_DIR}/templates/deployment.yaml.tmp <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    control-plane: controller-manager
  name: mystatefulset-controller-manager
  namespace: {{ .Release.Namespace }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      control-plane: controller-manager
  template:
    metadata:
      annotations:
        kubectl.kubernetes.io/default-container: manager
      labels:
        control-plane: controller-manager
    spec:
      containers:
        - name: manager
          args:
            - --leader-elect
          command:
            - /manager
          env:
            - name: ENABLE_WEBHOOKS
              value: "true"
          image: {{ .Values.image.repository }}:{{ .Values.image.tag }}
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8081
            initialDelaySeconds: 15
            periodSeconds: 20
          ports:
            - containerPort: 9443
              name: webhook-server
              protocol: TCP
          readinessProbe:
            httpGet:
              path: /readyz
              port: 8081
            initialDelaySeconds: 5
            periodSeconds: 10
          resources:
            {{- toYaml .Values.resources | nindent 12 }}
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
          volumeMounts:
            - mountPath: /tmp/k8s-webhook-server/serving-certs
              name: cert
              readOnly: true
      securityContext:
        runAsNonRoot: true
      serviceAccountName: mystatefulset-controller-manager
      terminationGracePeriodSeconds: 10
      volumes:
        - name: cert
          secret:
            defaultMode: 420
            secretName: webhook-server-cert
EOF

mv ${CHART_DIR}/templates/deployment.yaml.tmp ${CHART_DIR}/templates/deployment.yaml

# 清理临时文件
rm -f all.yaml

# 添加 NOTES.txt
cat >${CHART_DIR}/templates/NOTES.txt <<EOF
MyStatefulset Controller has been installed.

Check the controller status:
  kubectl get pods -n {{ .Release.Namespace }}

Check the CRD:
  kubectl get crd mystatefulsets.apps.mystatefulset.com

Check the webhooks:
  kubectl get validatingwebhookconfigurations
  kubectl get mutatingwebhookconfigurations
EOF

echo "Helm chart generated at ${CHART_DIR}"
