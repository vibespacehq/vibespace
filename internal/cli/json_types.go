package cli

import "github.com/vibespacehq/vibespace/pkg/jsonapi"

// Type aliases — all JSON wire types live in pkg/jsonapi for cross-package reuse.
type (
	JSONMeta                = jsonapi.JSONMeta
	JSONOutput              = jsonapi.JSONOutput
	JSONError               = jsonapi.JSONError
	VersionOutput           = jsonapi.VersionOutput
	ListOutput              = jsonapi.ListOutput
	VibespaceListItem       = jsonapi.VibespaceListItem
	StatusOutput            = jsonapi.StatusOutput
	ClusterStatus           = jsonapi.ClusterStatus
	ComponentStatus         = jsonapi.ComponentStatus
	DaemonStatus            = jsonapi.DaemonStatus
	DaemonVibespace         = jsonapi.DaemonVibespace
	AgentsOutput            = jsonapi.AgentsOutput
	AgentListItem           = jsonapi.AgentListItem
	ForwardsOutput          = jsonapi.ForwardsOutput
	AgentForwardInfo        = jsonapi.AgentForwardInfo
	ForwardInfo             = jsonapi.ForwardInfo
	SessionListOutput       = jsonapi.SessionListOutput
	SessionListItem         = jsonapi.SessionListItem
	SessionShowOutput       = jsonapi.SessionShowOutput
	SessionVibespace        = jsonapi.SessionVibespace
	DeleteOutput            = jsonapi.DeleteOutput
	CreateOutput            = jsonapi.CreateOutput
	ConfigShowOutput        = jsonapi.ConfigShowOutput
	ConfigShowAllOutput     = jsonapi.ConfigShowAllOutput
	AgentConfigItem         = jsonapi.AgentConfigItem
	AgentConfigOutput       = jsonapi.AgentConfigOutput
	InitOutput              = jsonapi.InitOutput
	StopOutput              = jsonapi.StopOutput
	AgentCreateOutput       = jsonapi.AgentCreateOutput
	AgentDeleteOutput       = jsonapi.AgentDeleteOutput
	StartOutput             = jsonapi.StartOutput
	ConfigSetOutput         = jsonapi.ConfigSetOutput
	ForwardAddOutput        = jsonapi.ForwardAddOutput
	ForwardRemoveOutput     = jsonapi.ForwardRemoveOutput
	SessionDeleteOutput     = jsonapi.SessionDeleteOutput
	MultiListSessionsOutput = jsonapi.MultiListSessionsOutput
	MultiSessionItem        = jsonapi.MultiSessionItem
	MultiListAgentsOutput   = jsonapi.MultiListAgentsOutput
	MultiMessageOutput      = jsonapi.MultiMessageOutput
	MultiRequestInfo        = jsonapi.MultiRequestInfo
	MultiAgentResponse      = jsonapi.MultiAgentResponse
	MultiToolUse            = jsonapi.MultiToolUse
	ExecOutput              = jsonapi.ExecOutput
	ServeOutput             = jsonapi.ServeOutput
	ServeTokenOutput        = jsonapi.ServeTokenOutput
	RemoteConnectOutput     = jsonapi.RemoteConnectOutput
	RemoteDisconnectOutput  = jsonapi.RemoteDisconnectOutput
	RemoteStatusOutput      = jsonapi.RemoteStatusOutput
	DiagnosticOutput        = jsonapi.DiagnosticOutput
	ClientListOutput        = jsonapi.ClientListOutput
	ClientOutput            = jsonapi.ClientOutput
)

// NewJSONOutput creates a new JSONOutput with metadata populated.
func NewJSONOutput(success bool, data interface{}, err *JSONError) JSONOutput {
	return jsonapi.NewJSONOutput(Version, success, data, err)
}
