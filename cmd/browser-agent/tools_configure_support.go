// tools_configure_support.go — Explicit preview-confirm export for privacy-bounded Doctor bundles.
package main

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"runtime"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statefile"
)

type doctorSupportArgs struct {
	Action            string `json:"doctor_action"`
	ConfirmationToken string `json:"confirmation_token"`
	OutputPath        string `json:"output_path"`
}

var writeDoctorSupportBundle = func(path string, data []byte) error { return statefile.Write(path, data, 0o600) }

func handleDoctorSupportAction(req mcp.JSONRPCRequest, args json.RawMessage, views []incident.DoctorView) (mcp.JSONRPCResponse, bool) {
	var input doctorSupportArgs
	if err := json.Unmarshal(args, &input); err != nil {
		slog.Warn("invalid Doctor support arguments", "component", "doctor_support_bundle", "stage", "decode")
		return mcp.Fail(req, mcp.ErrInvalidParam, "Invalid Doctor support arguments", "Pass a valid configure argument object"), true
	}
	if input.Action == "" {
		// EXPECTED_ABSENCE: Most configure calls are unrelated to support bundles, so no Doctor action is normal and must not create diagnostic noise.
		return mcp.JSONRPCResponse{}, false
	}
	if input.Action != "preview_support_bundle" && input.Action != "export_support_bundle" {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Unsupported doctor_action", "Use preview_support_bundle or export_support_bundle"), true
	}
	bundle := incident.BuildSupportBundle(version, runtime.GOOS+"-"+runtime.GOARCH, views)
	artifact, err := incident.SupportBundleBytes(bundle)
	if err != nil {
		slog.Error("Doctor support bundle encoding failed", "component", "doctor_support_bundle", "stage", "encode")
		return mcp.Fail(req, mcp.ErrInternal, "Support bundle encoding failed", "Retry and inspect local Doctor logs"), true
	}
	token, err := incident.SupportBundleToken(bundle)
	if err != nil {
		slog.Error("Doctor support bundle token failed", "component", "doctor_support_bundle", "stage", "encode")
		return mcp.Fail(req, mcp.ErrInternal, "Support bundle encoding failed", "Retry and inspect local Doctor logs"), true
	}
	preview := map[string]any{"artifact": string(artifact), "confirmation_token": token, "external_transmission": false}
	if input.Action == "preview_support_bundle" {
		return mcp.Succeed(req, "Doctor support bundle preview", preview), true
	}
	if input.OutputPath == "" || len(input.ConfirmationToken) != len(token) || subtle.ConstantTimeCompare([]byte(input.ConfirmationToken), []byte(token)) != 1 {
		// EXPECTED_ABSENCE: A missing or stale approval token is an expected validation rejection and does not represent a runtime incident.
		return mcp.Fail(req, mcp.ErrInvalidParam, "Support bundle confirmation does not match the current preview", "Preview again, then pass its confirmation_token and an explicit output_path"), true
	}
	if err := writeDoctorSupportBundle(input.OutputPath, artifact); err != nil {
		slog.Error("Doctor support bundle export failed", "component", "doctor_support_bundle", "stage", string(statefile.FailureStage(err)))
		if errors.Is(err, os.ErrPermission) {
			return mcp.Fail(req, mcp.ErrInternal, "Support bundle export failed", "Choose a writable local output_path"), true
		}
		return mcp.Fail(req, mcp.ErrInternal, "Support bundle export failed", "Inspect local Doctor logs and retry"), true
	}
	return mcp.Succeed(req, "Doctor support bundle exported locally", preview), true
}
