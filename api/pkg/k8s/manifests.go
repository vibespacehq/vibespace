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

// Local Registry manifest
//
//go:embed manifests/registry/registry.yaml
var RegistryManifest []byte

// BuildKit manifest
//
//go:embed manifests/buildkit/buildkit.yaml
var BuildKitManifest []byte
