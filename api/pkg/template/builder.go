package template

import (
	_ "embed"
)

// Embedded support files for building images

//go:embed images/config/vscode-settings.json
var vscodeSettingsData []byte

//go:embed images/base/Caddyfile
var caddyfileData []byte

// Shared template files (used by all templates: nextjs, vue, jupyter)
//go:embed images/supervisord.conf
var supervisordConfData []byte

//go:embed images/entrypoint.sh
var entrypointShData []byte

// Next.js template files
//go:embed images/templates/nextjs/preview.sh
var nextjsPreviewShData []byte

//go:embed images/templates/nextjs/prod.sh
var nextjsProdShData []byte

// Vue template files
//go:embed images/templates/vue/preview.sh
var vuePreviewShData []byte

//go:embed images/templates/vue/prod.sh
var vueProdShData []byte

// Jupyter template files
//go:embed images/templates/jupyter/preview.sh
var jupyterPreviewShData []byte

//go:embed images/templates/jupyter/prod.sh
var jupyterProdShData []byte

// GetAllSupportFiles returns all support files needed for building images
// These are config files, scripts, etc. that are COPYed into containers
func GetAllSupportFiles() map[string][]byte {
	return map[string][]byte{
		// Base image support files
		"vscode-settings.json": vscodeSettingsData,
		"Caddyfile":            caddyfileData,
		// Template support files (shared)
		"supervisord.conf": supervisordConfData,
		"entrypoint.sh":    entrypointShData,
		// Next.js template scripts
		"nextjs-preview.sh": nextjsPreviewShData,
		"nextjs-prod.sh":    nextjsProdShData,
		// Vue template scripts
		"vue-preview.sh": vuePreviewShData,
		"vue-prod.sh":    vueProdShData,
		// Jupyter template scripts
		"jupyter-preview.sh": jupyterPreviewShData,
		"jupyter-prod.sh":    jupyterProdShData,
	}
}
