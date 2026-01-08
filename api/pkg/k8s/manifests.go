package k8s

import _ "embed"

// Knative Serving manifests
//
//go:embed manifests/knative/serving-crds.yaml
var KnativeServingCRDs []byte

//go:embed manifests/knative/serving-core.yaml
var KnativeServingCore []byte

// Traefik Ingress manifests
//
//go:embed manifests/traefik/traefik-crds.yaml
var TraefikCRDs []byte

//go:embed manifests/traefik/traefik.yaml
var TraefikManifest []byte

// BuildKit manifest
//
//go:embed manifests/buildkit/buildkit.yaml
var BuildKitManifest []byte

// Docker Registry manifest (simple registry for mirrored images and custom builds)
//
//go:embed manifests/registry/registry.yaml
var RegistryManifest []byte
