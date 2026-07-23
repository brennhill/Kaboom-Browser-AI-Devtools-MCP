/**
 * Purpose: Scans page forms to extract field metadata (name, type, required, validation) and optionally runs client-side validation.
 * Docs: docs/features/feature/form-filling/index.md
 */
interface FormDiscoveryParams {
    selector?: string;
    mode?: 'discover' | 'validate';
}
interface FormFieldInfo {
    name: string;
    type: string;
    required: boolean;
    value: string;
    label: string;
    selector: string;
    tag: string;
    validation_constraints: Record<string, string | number | boolean>;
    options?: Array<{
        value: string;
        text: string;
        selected: boolean;
    }>;
    validation_message?: string;
}
interface FormInfo {
    action: string;
    method: string;
    selector: string;
    id: string;
    name: string;
    fields: FormFieldInfo[];
    submit_button: {
        selector: string;
        text: string;
    } | null;
    valid?: boolean;
    validation_errors?: Array<{
        field: string;
        message: string;
    }>;
}
/**
 * Discover forms on the page.
 */
export declare function discoverForms(params: FormDiscoveryParams): FormInfo[];
export {};
//# sourceMappingURL=form-discovery.d.ts.map