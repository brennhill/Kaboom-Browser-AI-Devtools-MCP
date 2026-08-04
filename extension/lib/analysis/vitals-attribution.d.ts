/**
 * Purpose: Build bounded, content-free attribution for browser Web Vitals entries.
 * Docs: docs/features/feature/web-vitals/index.md
 */
export interface ElementDescriptor {
    tag: string;
    id?: string;
    classes?: string[];
    role?: string;
}
export interface LCPAttribution {
    element?: ElementDescriptor;
    time_to_first_byte_ms: number;
    resource_load_delay_ms: number;
    resource_load_duration_ms: number;
    element_render_delay_ms: number;
    attribution_status: 'available' | 'element_unavailable';
}
export interface INPAttribution {
    event_type: string;
    target?: ElementDescriptor;
    input_delay_ms: number;
    processing_ms: number;
    presentation_delay_ms: number;
    interaction_id: number;
}
export interface CLSAttribution {
    shifts: Array<{
        value: number;
        start_time: number;
        nodes: ElementDescriptor[];
    }>;
    attribution_status: 'available' | 'nodes_unavailable';
}
export interface LongTaskAttribution {
    name: string;
    start_time: number;
    duration: number;
    source_stack?: string[];
    source_stack_status: 'available' | 'unavailable';
}
export interface VitalsAttribution {
    lcp?: LCPAttribution;
    inp?: INPAttribution;
    cls: CLSAttribution;
    long_tasks: LongTaskAttribution[];
}
export declare function resetVitalsAttribution(): void;
export declare function recordLCPAttribution(entry: PerformanceEntry): void;
export declare function recordINPAttribution(entry: PerformanceEntry): void;
export declare function recordCLSAttribution(entry: PerformanceEntry): void;
export declare function recordLongTaskAttribution(entry: PerformanceEntry): void;
export declare function getVitalsAttribution(responseStart?: number): VitalsAttribution;
//# sourceMappingURL=vitals-attribution.d.ts.map