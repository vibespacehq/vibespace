#!/bin/bash
# k3s installation script for Workspace

set -e

echo "================================================"
echo "Workspace - k3s Installation"
echo "================================================"
echo ""

# Check if k3s is already installed
if command -v k3s &> /dev/null; then
    echo "✓ k3s is already installed"
    k3s --version
    read -p "Reinstall? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Skipping installation."
        exit 0
    fi
fi

echo "Installing k3s..."
curl -sfL https://get.k3s.io | sh -s - \
  --write-kubeconfig-mode 644 \
  --disable traefik \
  --disable servicelb \
  --kube-apiserver-arg=feature-gates=ServerSideApply=true

echo ""
echo "Waiting for k3s to be ready..."
until kubectl get nodes 2>/dev/null | grep -q Ready; do
  sleep 2
  echo -n "."
done
echo ""
echo "✓ k3s is ready"
echo ""

echo "Installing Knative Serving..."
kubectl apply -f https://github.com/knative/serving/releases/download/knative-v1.11.0/serving-crds.yaml
kubectl apply -f https://github.com/knative/serving/releases/download/knative-v1.11.0/serving-core.yaml
echo "✓ Knative Serving installed"
echo ""

# Check if manifests exist
if [ -d "../k8s" ]; then
    echo "Installing cluster components..."
    kubectl apply -f ../k8s/traefik.yaml
    kubectl apply -f ../k8s/registry.yaml
    kubectl apply -f ../k8s/buildkit.yaml
    echo "✓ Cluster components installed"
    echo ""
fi

echo "Waiting for all pods to be ready..."
kubectl wait --for=condition=ready pod --all --all-namespaces --timeout=5m || true
echo ""

echo "================================================"
echo "Installation Complete!"
echo "================================================"
echo ""
echo "Cluster Status:"
kubectl get nodes
echo ""
echo "Pods:"
kubectl get pods --all-namespaces
echo ""
echo "Next steps:"
echo "  1. cd app && npm install && npm run tauri:dev"
echo "  2. Create your first workspace"
echo ""
