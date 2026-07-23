/** A step in a property/method chain */
export type StructuredStep = {
    op: 'access';
    key: string;
} | {
    op: 'index';
    index: number;
} | {
    op: 'call';
    args: StructuredValue[];
} | {
    op: 'construct';
    args: StructuredValue[];
};
/** A value: literal, global reference, or chain of steps */
export type StructuredValue = {
    type: 'literal';
    value: string | number | boolean | null;
} | {
    type: 'undefined';
} | {
    type: 'global';
    name: string;
} | {
    type: 'chain';
    root: StructuredValue;
    steps: StructuredStep[];
} | {
    type: 'array';
    elements: StructuredValue[];
} | {
    type: 'object';
    entries: Array<{
        key: string;
        value: StructuredValue;
    }>;
};
/** A structured command representing a single JS expression, optionally with assignment */
export interface StructuredCommand {
    expr: StructuredValue;
    assign?: {
        target: StructuredValue;
        steps: StructuredStep[];
        key: string;
    };
}
/** Result of parsing: either success with a command, or failure with a reason */
export type ParseResult = {
    ok: true;
    command: StructuredCommand;
} | {
    ok: false;
    reason: string;
};
//# sourceMappingURL=csp-safe-types.d.ts.map