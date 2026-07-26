// Purpose: Re-exports canonical WebSocket type aliases for capture package compatibility.
// Why: Keeps capture call sites stable while canonical type ownership lives in internal/types.
// Docs: docs/features/feature/normalized-event-schema/index.md

package capture

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// WebSocketEvent is an alias to canonical definition in internal/types/network.go
type WebSocketEvent = types.WebSocketEvent

// SamplingInfo is an alias to canonical definition in internal/types/network.go
type SamplingInfo = types.SamplingInfo

// WebSocketEventFilter is an alias to canonical definition in internal/types/network.go
type WebSocketEventFilter = types.WebSocketEventFilter

// WebSocketStatusFilter is an alias to canonical definition in internal/types/network.go
type WebSocketStatusFilter = types.WebSocketStatusFilter

// WebSocketStatusResponse is an alias to canonical definition in internal/types/network.go
type WebSocketStatusResponse = types.WebSocketStatusResponse

// WebSocketConnection is an alias to canonical definition in internal/types/network.go
type WebSocketConnection = types.WebSocketConnection

// WebSocketClosedConnection is an alias to canonical definition in internal/types/network.go
type WebSocketClosedConnection = types.WebSocketClosedConnection

// WebSocketMessageRate is an alias to canonical definition in internal/types/network.go
type WebSocketMessageRate = types.WebSocketMessageRate

// WebSocketDirectionStats is an alias to canonical definition in internal/types/network.go
type WebSocketDirectionStats = types.WebSocketDirectionStats

// WebSocketLastMessage is an alias to canonical definition in internal/types/network.go
type WebSocketLastMessage = types.WebSocketLastMessage

// WebSocketMessagePreview is an alias to canonical definition in internal/types/network.go
type WebSocketMessagePreview = types.WebSocketMessagePreview

// WebSocketSchema is an alias to canonical definition in internal/types/network.go
type WebSocketSchema = types.WebSocketSchema

// WebSocketSamplingStatus is an alias to canonical definition in internal/types/network.go
type WebSocketSamplingStatus = types.WebSocketSamplingStatus
