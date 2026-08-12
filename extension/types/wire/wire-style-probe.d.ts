/**
 * @fileoverview Wire types — matches internal/styleprobe/wire_style_probe.go
 *
 * Canonical TypeScript definitions for the wire payloads.
 * Changes here MUST be mirrored in the Go counterpart. Run `make check-wire-drift`.
 */
/**
 * WireStyleProbeResult is the payload the page returns for one probe query.
 */
export interface WireStyleProbeResult {
    readonly elements: readonly WireStyleProbeElement[];
    readonly count: number;
    readonly match_count: number;
    readonly truncated: boolean;
    readonly root_tokens?: Readonly<Record<string, string>>;
}
/**
 * WireStyleProbeElement is one matched element's observed state.
 */
export interface WireStyleProbeElement {
    readonly selector: string;
    readonly tag: string;
    readonly computed_styles: Readonly<Record<string, string>>;
    readonly box_model: WireStyleProbeBox;
    readonly contrast_ratio?: number;
    readonly custom_properties?: Readonly<Record<string, string>>;
    readonly index: number;
    readonly parent_display?: string;
    readonly parent_gap?: string;
    readonly in_flow: boolean;
}
/**
 * WireStyleProbeBox is the element's border-box geometry in viewport pixels.
 */
export interface WireStyleProbeBox {
    readonly x: number;
    readonly y: number;
    readonly width: number;
    readonly height: number;
    readonly top: number;
    readonly right: number;
    readonly bottom: number;
    readonly left: number;
}
//# sourceMappingURL=wire-style-probe.d.ts.map