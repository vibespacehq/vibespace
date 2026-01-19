package k8s

import _ "embed"

// Knative Serving manifests
//
//go:embed manifests/knative/serving-crds.yaml
var KnativeServingCRDs []byte

//go:embed manifests/knative/serving-core.yaml
var KnativeServingCore []byte

// Docker Registry manifest
//
//go:embed manifests/registry/registry.yaml
var RegistryManifest []byte
